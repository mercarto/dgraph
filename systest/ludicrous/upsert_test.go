package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/dgo/v200/protos/api"
	"github.com/dgraph-io/dgraph/testutil"
	"github.com/stretchr/testify/require"
)

func InitData(t *testing.T) {
	dg, err := testutil.DgraphClient(testutil.SockAddr)
	require.NoError(t, err)
	schema := `
		name: string @index(exact) .
		email: string @index(exact) .
		count: [int]  .
	`

	err = dg.Alter(context.Background(), &api.Operation{Schema: schema})
	require.NoError(t, err)
	type Person struct {
		Name  string `json:"name,omitempty"`
		count int
	}

	p := Person{
		Name:  "Alice",
		count: 1,
	}
	pb, err := json.Marshal(p)
	require.NoError(t, err)

	mu := &api.Mutation{
		SetJson:   pb,
		CommitNow: true,
	}
	txn := dg.NewTxn()
	ctx := context.Background()
	defer txn.Discard(ctx)

	res, err := txn.Mutate(ctx, mu)
	require.NoError(t, err)
	fmt.Printf("string(res = %+v\n", res)
}

func TestConcurrentUpdate(t *testing.T) {
	InitData(t)
	ctx := context.Background()
	dg, err := testutil.DgraphClient(testutil.SockAddr)
	require.NoError(t, err)
	testutil.DropAll(t, dg)

	count := 10
	var wg sync.WaitGroup
	wg.Add(count)
	mutation := func(i int) {
		defer wg.Done()
		// RETRY:
		query := fmt.Sprintf(`query {
			user as var(func: eq(name, "Alice"))
			}`)
		mu := &api.Mutation{
			SetNquads: []byte(fmt.Sprintf(`uid(user) <count> "%d" .`, i)),
		}
		req := &api.Request{
			Query:     query,
			Mutations: []*api.Mutation{mu},
			CommitNow: true,
		}
		// Update email only if matching uid found.
		_, err := dg.NewTxn().Do(ctx, req)
		require.NoError(t, err)
		// if err == dgo.ErrAborted {
		// 	goto RETRY
		// }
	}
	for i := 0; i < count; i++ {
		go mutation(i)
	}
	wg.Wait()
	time.Sleep(time.Second * 2)
	for i := 0; i < 10; i++ {
		q := `query all($a: string) {
			all(func: eq(name, $a)) {
			  name
			  dgraph.type
			  count
			}
		  }`

		txn := dg.NewTxn()
		res, err := txn.QueryWithVars(ctx, q, map[string]string{"$a": "Alice"})
		require.NoError(t, err)
		fmt.Printf("%s\n", res.Json)
	}
}

func TestSequentialUpdate(t *testing.T) {
	InitData(t)
	ctx := context.Background()
	dg, err := testutil.DgraphClient(testutil.SockAddr)
	require.NoError(t, err)
	testutil.DropAll(t, dg)

	count := 10
	mutation := func(i int) {
		query := fmt.Sprintf(`query {
			user as var(func: eq(name, "Alice"))
			}`)
		mu := &api.Mutation{
			SetNquads: []byte(fmt.Sprintf(`uid(user) <count> "%d" .`, i)),
		}
		req := &api.Request{
			Query:     query,
			Mutations: []*api.Mutation{mu},
			CommitNow: true,
		}
		_, err := dg.NewTxn().Do(ctx, req)
		require.NoError(t, err)

	}
	for i := 0; i < count; i++ {
		mutation(i)
	}
	time.Sleep(time.Second * 2)
	for i := 0; i < 10; i++ {
		q := `query all($a: string) {
			all(func: eq(name, $a)) {
			  name
			  dgraph.type
			  count
			}
		  }`

		txn := dg.NewTxn()
		res, err := txn.QueryWithVars(ctx, q, map[string]string{"$a": "Alice"})
		require.NoError(t, err)
		fmt.Printf("%s\n", res.Json)
	}
}
