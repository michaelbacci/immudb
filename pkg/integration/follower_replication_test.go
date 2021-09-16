/*
Copyright 2021 CodeNotary, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/codenotary/immudb/pkg/api/schema"
	ic "github.com/codenotary/immudb/pkg/client"
	"github.com/codenotary/immudb/pkg/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestReplication(t *testing.T) {
	//init master server
	masterServerOpts := server.DefaultOptions().
		WithMetricsServer(false).
		WithWebServer(false).
		WithPgsqlServer(false).
		WithPort(0).
		WithDir("master-data")

	masterServer := server.DefaultServer().WithOptions(masterServerOpts).(*server.ImmuServer)
	defer os.RemoveAll(masterServerOpts.Dir)

	err := masterServer.Initialize()
	require.NoError(t, err)

	//init follower server
	followerServerOpts := server.DefaultOptions().
		WithMetricsServer(false).
		WithWebServer(false).
		WithPgsqlServer(false).
		WithPort(0).
		WithDir("follower-data")

	followerServer := server.DefaultServer().WithOptions(followerServerOpts).(*server.ImmuServer)
	defer os.RemoveAll(followerServerOpts.Dir)

	err = followerServer.Initialize()
	require.NoError(t, err)

	go func() {
		masterServer.Start()
	}()

	go func() {
		followerServer.Start()
	}()

	time.Sleep(500 * time.Millisecond)

	defer masterServer.Stop()
	defer followerServer.Stop()

	// init master client
	masterPort := masterServer.Listener.Addr().(*net.TCPAddr).Port
	masterClient, err := ic.NewImmuClient(ic.DefaultOptions().WithPort(masterPort))
	require.NoError(t, err)
	require.NotNil(t, masterClient)

	lr, err := masterClient.Login(context.TODO(), []byte(`immudb`), []byte(`immudb`))
	require.NoError(t, err)

	md := metadata.Pairs("authorization", lr.Token)
	mctx := metadata.NewOutgoingContext(context.Background(), md)

	// init follower client
	followerPort := followerServer.Listener.Addr().(*net.TCPAddr).Port
	followerClient := ic.DefaultClient().WithOptions(ic.DefaultOptions().WithPort(followerPort))
	require.NotNil(t, followerClient)

	lr, err = followerClient.Login(context.TODO(), []byte(`immudb`), []byte(`immudb`))
	require.NoError(t, err)

	md = metadata.Pairs("authorization", lr.Token)
	fctx := metadata.NewOutgoingContext(context.Background(), md)

	// create database as replica in follower server
	err = followerClient.CreateDatabase(fctx, &schema.DatabaseSettings{
		DatabaseName:   "replicateddb",
		Replica:        true,
		MasterDatabase: "defaultdb",
		MasterAddress:  "127.0.0.1",
		MasterPort:     uint32(masterPort),
	})
	require.NoError(t, err)

	fdb, err := followerClient.UseDatabase(fctx, &schema.Database{DatabaseName: "replicateddb"})
	require.NoError(t, err)
	require.NotNil(t, fdb)

	md = metadata.Pairs("authorization", fdb.Token)
	fctx = metadata.NewOutgoingContext(context.Background(), md)

	t.Run("key1 should not exist", func(t *testing.T) {
		_, err = followerClient.Get(fctx, []byte("key1"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "key not found")
	})

	_, err = masterClient.Set(mctx, []byte("key1"), []byte("value1"))
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	t.Run("key1 should exist", func(t *testing.T) {
		_, err = followerClient.Get(fctx, []byte("key1"))
		require.NoError(t, err)
	})
}