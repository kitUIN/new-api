package model

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const longChannelGroups = "codex-pro-正价菲区,codex-pro-正价luna,grok-正价,codex-pro正价,codex-pro-正价,codex-plus-正价luna"

func TestChannelGroupColumnSupportsMembershipLists(t *testing.T) {
	for _, dialect := range []gorm.Dialector{
		sqlite.Open(":memory:"),
		mysql.New(mysql.Config{SkipInitializeWithVersion: true}),
		postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}),
	} {
		t.Run(dialect.Name(), func(t *testing.T) {
			db, err := gorm.Open(dialect, &gorm.Config{DisableAutomaticPing: true})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			statement := &gorm.Statement{DB: db}
			require.NoError(t, statement.Parse(&Channel{}))
			field := statement.Schema.LookUpField("Group")
			require.NotNil(t, field)
			columnType := strings.ToUpper(db.Migrator().FullDataTypeOf(field).SQL)
			require.Equal(t, "TEXT", columnType, "membership lists must not have a 64-character limit or a default group; MySQL 5.7 also forbids TEXT defaults")
		})
	}
}

func TestChannelGroupAutoMigratePreservesMemberships(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	legacy := struct {
		Id    int
		Key   string `gorm:"not null"`
		Group string `gorm:"type:varchar(64);default:'default'"`
	}{Id: 1, Key: "test-key", Group: "default,existing"}
	require.NoError(t, db.Table("channels").AutoMigrate(&legacy))
	require.NoError(t, db.Table("channels").Create(&legacy).Error)

	for i := 0; i < 2; i++ {
		require.NoError(t, db.AutoMigrate(&Channel{}))
	}
	var stored Channel
	require.NoError(t, db.First(&stored, legacy.Id).Error)
	require.Equal(t, legacy.Group, stored.Group)
	require.Equal(t, legacy.Key, stored.Key)
	require.NoError(t, db.Model(&stored).Update("group", longChannelGroups).Error)
	require.NoError(t, db.First(&stored, legacy.Id).Error)
	require.Equal(t, longChannelGroups, stored.Group)

	created := Channel{Key: "new-key"}
	require.NoError(t, db.Create(&created).Error)
	require.NoError(t, db.First(&created, created.Id).Error)
	require.Empty(t, created.Group, "the legacy default group must not be applied to new channels")
	columns, err := db.Migrator().ColumnTypes(&Channel{})
	require.NoError(t, err)
	for _, column := range columns {
		if column.Name() == "group" {
			require.Equal(t, "text", strings.ToLower(column.DatabaseTypeName()))
			_, hasDefault := column.DefaultValue()
			require.False(t, hasDefault)
			return
		}
	}
	t.Fatal("group column missing")
}

func TestSetGroupChannelsSupportsLongMembershipLists(t *testing.T) {
	truncateTables(t)
	channel := Channel{
		Name: "unassigned", Key: "test-key", Status: common.ChannelStatusEnabled,
		Models: "claude-sonnet-5-5,claude-haiku-5-5,claude-opus-5-5",
	}
	require.NoError(t, channel.Insert())
	require.Empty(t, channel.Group)
	var abilities []Ability
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.Empty(t, abilities)

	require.Equal(t, 82, utf8.RuneCountInString(longChannelGroups))
	groups := strings.Split(longChannelGroups, ",")
	for _, group := range groups {
		require.NoError(t, SetGroupChannels(group, []int{channel.Id}))
	}
	require.NoError(t, DB.First(&channel, channel.Id).Error)
	require.Equal(t, longChannelGroups, channel.Group)
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.Len(t, abilities, 18)
	for _, group := range groups {
		for _, model := range channel.GetModels() {
			require.True(t, IsChannelEnabledForGroupModel(group, model, channel.Id))
		}
		require.NoError(t, SetGroupChannels(group, nil))
	}
	require.NoError(t, DB.First(&channel, channel.Id).Error)
	require.Empty(t, channel.Group)
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.Empty(t, abilities)
}
