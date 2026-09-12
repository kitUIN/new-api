import {
  InfiniteQueryObserver,
  QueryClient,
  QueryObserver,
} from '@tanstack/react-query'
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  invalidateTicketQueries,
  refreshClosedTicketMessages,
  refreshTicketUnreadCount,
} from './queries'

function createClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity, gcTime: Infinity },
    },
  })
}

describe('ticket message refresh', () => {
  test('fetches the final reply while retaining loaded history pages', async (t) => {
    const client = createClient()
    let messages = [1, 2, 3]
    const observer = new InfiniteQueryObserver(client, {
      queryKey: ['tickets', 1, 'messages', 7],
      queryFn: async ({ pageParam }) => {
        const older = messages.filter((id) => pageParam === 0 || id < pageParam)
        return { items: older.slice(-2), has_more: older.length > 2 }
      },
      initialPageParam: 0,
      getNextPageParam: (last) => (last.has_more ? last.items[0] : undefined),
    })
    const unsubscribe = observer.subscribe(() => {})
    t.after(() => {
      unsubscribe()
      client.clear()
    })
    await observer.refetch()
    await observer.fetchNextPage()
    messages = [1, 2, 3, 4]

    await refreshClosedTicketMessages(client, 1, 7)

    const pages = observer.getCurrentResult().data?.pages
    assert.equal(pages?.length, 2)
    assert.deepEqual(
      pages?.flatMap((page) => page.items),
      [3, 4, 1, 2]
    )
  })

  for (const initialLoad of [true, false]) {
    test(`waits for a stale ${initialLoad ? 'initial request' : 'poll'} before fetching the final reply`, async (t) => {
      const client = createClient()
      let serverMessages = [1]
      let holdRequest = initialLoad
      let releaseRequest: () => void = () => {}
      const pending = new Promise<void>((resolve) => {
        releaseRequest = resolve
      })
      let fetches = 0
      const observer = new InfiniteQueryObserver(client, {
        queryKey: ['tickets', 1, 'messages', 7],
        queryFn: async () => {
          fetches++
          const items = [...serverMessages]
          if (holdRequest) await pending
          return { items }
        },
        initialPageParam: 0,
        getNextPageParam: () => undefined,
      })
      const unsubscribe = observer.subscribe(() => {})
      t.after(() => {
        unsubscribe()
        client.clear()
      })
      if (!initialLoad) await observer.refetch()
      holdRequest = true
      const oldRequest = observer.refetch()
      serverMessages = [1, 2]
      const closingRefresh = refreshClosedTicketMessages(client, 1, 7)
      releaseRequest()
      await oldRequest
      await closingRefresh

      assert.deepEqual(observer.getCurrentResult().data?.pages[0].items, [1, 2])
      assert.equal(fetches, initialLoad ? 2 : 3)
    })
  }

  test('refreshes ticket data without downloading immutable images again', async (t) => {
    const client = createClient()
    const reads = { list: 0, detail: 0, messages: 0, image: 0, otherUser: 0 }
    const subscriptions: Array<() => void> = []
    t.after(() => {
      subscriptions.forEach((unsubscribe) => unsubscribe())
      client.clear()
    })
    for (const kind of [
      'list',
      'detail',
      'messages',
      'image',
      'otherUser',
    ] as const) {
      const observer = new QueryObserver(client, {
        queryKey: ['tickets', kind === 'otherUser' ? 2 : 1, kind, 7],
        queryFn: async () => ++reads[kind],
      })
      subscriptions.push(observer.subscribe(() => {}))
      await observer.refetch()
    }

    await invalidateTicketQueries(client, 1)

    assert.deepEqual(reads, {
      list: 2,
      detail: 2,
      messages: 2,
      image: 1,
      otherUser: 1,
    })
  })

  test('discards an old unread poll after reading without clearing another account', async (t) => {
    const client = createClient()
    let count = 3
    let holdRequest = false
    let releaseRequest: () => void = () => {}
    const pending = new Promise<void>((resolve) => {
      releaseRequest = resolve
    })
    let oldSignal: AbortSignal | undefined
    const observer = new QueryObserver(client, {
      queryKey: ['tickets', 1, 'unread'],
      queryFn: async ({ signal }) => {
        const snapshot = count
        if (holdRequest) {
          oldSignal = signal
          await pending
        }
        return snapshot
      },
    })
    const unsubscribe = observer.subscribe(() => {})
    t.after(() => {
      unsubscribe()
      client.clear()
    })
    client.setQueryData(['tickets', 2, 'unread'], 9)
    await observer.refetch()
    holdRequest = true
    const oldPoll = observer.refetch()
    count = 1 // Only the displayed ticket was read; another still has a reply.
    holdRequest = false

    await refreshTicketUnreadCount(client, 1)
    assert.equal(oldSignal?.aborted, true)
    assert.equal(observer.getCurrentResult().data, 1)
    releaseRequest()
    await oldPoll
    assert.equal(observer.getCurrentResult().data, 1)
    assert.equal(client.getQueryData(['tickets', 2, 'unread']), 9)
  })
})
