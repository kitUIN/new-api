package model

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSessionSecretTest(t *testing.T) string {
	t.Helper()
	previousDB := DB
	previousSession, previousCrypto := common.SessionSecret, common.CryptoSecret
	t.Cleanup(func() {
		DB = previousDB
		common.SessionSecret, common.CryptoSecret = previousSession, previousCrypto
	})
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("CRYPTO_SECRET", "")
	path := filepath.Join(t.TempDir(), "session.db")
	DB = openSessionSecretTestDB(t, path)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	return path
}

func openSessionSecretTestDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func newSessionSecretTestServer() *gin.Engine {
	server := gin.New()
	store := cookie.NewStore([]byte(common.SessionSecret))
	store.Options(sessions.Options{Path: "/", MaxAge: 2592000, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	server.Use(sessions.Sessions("session", store))
	server.POST("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 42)
		session.Set("username", "session-test")
		if err := session.Save(); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	server.GET("/self", func(c *gin.Context) {
		session := sessions.Default(c)
		if session.Get("id") != 42 || session.Get("username") != "session-test" {
			c.Status(http.StatusUnauthorized)
			return
		}
		c.Status(http.StatusOK)
	})
	return server
}

func TestSessionCookieSurvivesRestart(t *testing.T) {
	path := setupSessionSecretTest(t)
	require.NoError(t, InitSessionSecret())
	secret := common.SessionSecret
	require.Len(t, secret, 64)
	hmac := common.GenerateHMAC("cache-key")

	login := httptest.NewRecorder()
	newSessionSecretTestServer().ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/login", nil))
	require.Equal(t, http.StatusOK, login.Code)
	cookies := login.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, 2592000, cookies[0].MaxAge)
	require.True(t, cookies[0].HttpOnly)

	// Discard process state, reopen the persisted database, and recreate the store.
	sqlDB, err := DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	common.SessionSecret, common.CryptoSecret = "", ""
	DB = openSessionSecretTestDB(t, path)
	require.NoError(t, InitSessionSecret())
	require.Equal(t, secret, common.SessionSecret)
	require.Equal(t, hmac, common.GenerateHMAC("cache-key"))

	request := httptest.NewRequest(http.MethodGet, "/self", nil)
	request.AddCookie(cookies[0])
	response := httptest.NewRecorder()
	newSessionSecretTestServer().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)

	// A deliberate key rotation must still invalidate the old login.
	t.Setenv("SESSION_SECRET", "different-explicit-session-secret-for-test")
	require.NoError(t, InitSessionSecret())
	request = httptest.NewRequest(http.MethodGet, "/self", nil)
	request.AddCookie(cookies[0])
	response = httptest.NewRecorder()
	newSessionSecretTestServer().ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestSessionSecretEnvironmentOverrides(t *testing.T) {
	setupSessionSecretTest(t)
	require.NoError(t, InitSessionSecret())
	savedSecret := common.SessionSecret

	t.Setenv("SESSION_SECRET", "configured-session-secret")
	t.Setenv("CRYPTO_SECRET", "configured-crypto-secret")
	require.NoError(t, InitSessionSecret())
	require.Equal(t, "configured-session-secret", common.SessionSecret)
	require.Equal(t, "configured-crypto-secret", common.CryptoSecret)

	t.Setenv("SESSION_SECRET", "")
	require.NoError(t, InitSessionSecret())
	require.Equal(t, savedSecret, common.SessionSecret)
	require.Equal(t, "configured-crypto-secret", common.CryptoSecret)

	// An explicit key does not require reading or writing the options table.
	DB = nil
	t.Setenv("SESSION_SECRET", "configured-session-secret")
	t.Setenv("CRYPTO_SECRET", "")
	require.NoError(t, InitSessionSecret())
	require.Equal(t, "configured-session-secret", common.SessionSecret)
	require.Equal(t, common.SessionSecret, common.CryptoSecret)
	t.Setenv("SESSION_SECRET", "random_string")
	require.Error(t, InitSessionSecret())
}

func TestSessionSecretConcurrentInitialization(t *testing.T) {
	setupSessionSecretTest(t)
	const nodes = 8
	secrets := make([]string, nodes)
	errors := make([]error, nodes)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			secrets[i], errors[i] = loadOrCreateSessionSecret(DB)
		}()
	}
	close(start)
	wg.Wait()
	for i := range nodes {
		require.NoError(t, errors[i])
		require.NotEmpty(t, secrets[i])
		require.Equal(t, secrets[0], secrets[i])
	}
	var count int64
	require.NoError(t, DB.Model(&Option{}).Where(&Option{Key: sessionSecretOptionKey}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestSessionSecretIsInternalOption(t *testing.T) {
	setupSessionSecretTest(t)
	require.NoError(t, InitSessionSecret())
	secret := common.SessionSecret
	require.NoError(t, DB.Create(&Option{Key: "SystemName", Value: "test"}).Error)
	options, err := AllOption()
	require.NoError(t, err)
	require.Len(t, options, 1)
	require.Equal(t, "SystemName", options[0].Key)
	for _, key := range []string{sessionSecretOptionKey, "sessionsecret", "SESSIONSECRET", "SessionSecret "} {
		require.Error(t, UpdateOption(key, "replacement"))
	}
	saved, err := loadOrCreateSessionSecret(DB)
	require.NoError(t, err)
	require.Equal(t, secret, saved)
}

func TestSessionSecretFailsOnInvalidStorage(t *testing.T) {
	setupSessionSecretTest(t)
	common.SessionSecret, common.CryptoSecret = "", ""
	for _, value := range []string{"", "random_string"} {
		require.NoError(t, DB.Save(&Option{Key: sessionSecretOptionKey, Value: value}).Error)
		require.Error(t, InitSessionSecret())
		require.Empty(t, common.SessionSecret)
		require.Empty(t, common.CryptoSecret)
		var saved Option
		require.NoError(t, DB.Where(&Option{Key: sessionSecretOptionKey}).Take(&saved).Error)
		require.Equal(t, value, saved.Value)
	}
	require.NoError(t, DB.Migrator().DropTable(&Option{}))
	require.Error(t, InitSessionSecret())
	require.Empty(t, common.SessionSecret)
}
