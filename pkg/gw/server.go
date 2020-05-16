/*
Copyright 2019-2020 vChain, Inc.

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

package gw

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/codenotary/immudb/pkg/api/schema"
	immuclient "github.com/codenotary/immudb/pkg/client"
	"github.com/codenotary/immudb/pkg/server"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/rs/cors"
)

func (s *ImmuGwServer) Start() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cliOpts := &immuclient.Options{
		Dir:                s.Options.Dir,
		Address:            s.Options.ImmudbAddress,
		Port:               s.Options.ImmudbPort,
		HealthCheckRetries: 1,
		MTLs:               s.Options.MTLs,
		MTLsOptions:        s.Options.MTLsOptions,
		Auth:               false,
		Config:             "",
	}

	ic, err := immuclient.NewImmuClient(cliOpts)
	if err != nil {
		s.Logger.Errorf("unable to instantiate client: %s", err)
		return err
	}
	mux := runtime.NewServeMux()

	handler := cors.Default().Handler(mux)

	sh := NewSetHandler(mux, ic, s.RootService)
	ssh := NewSafesetHandler(mux, ic, s.RootService)
	sgh := NewSafegetHandler(mux, ic, s.RootService)
	hh := NewHistoryHandler(mux, ic, s.RootService)
	sr := NewSafeReferenceHandler(mux, ic, s.RootService)
	sza := NewSafeZAddHandler(mux, ic, s.RootService)

	mux.Handle(http.MethodPost, schema.Pattern_ImmuService_Set_0(), sh.Set)
	mux.Handle(http.MethodPost, schema.Pattern_ImmuService_SafeSet_0(), ssh.Safeset)
	mux.Handle(http.MethodPost, schema.Pattern_ImmuService_SafeGet_0(), sgh.Safeget)
	mux.Handle(http.MethodGet, schema.Pattern_ImmuService_History_0(), hh.History)
	mux.Handle(http.MethodPost, schema.Pattern_ImmuService_SafeReference_0(), sr.SafeReference)
	mux.Handle(http.MethodPost, schema.Pattern_ImmuService_SafeZAdd_0(), sza.SafeZAdd)

	err = schema.RegisterImmuServiceHandlerClient(ctx, mux, *ic.GetServiceClient())
	if err != nil {
		s.Logger.Errorf("unable to register client handlers: %s", err)
		return err
	}

	s.installShutdownHandler()
	s.Logger.Infof("starting immugw: %v", s.Options)
	if s.Options.Pidfile != "" {
		if s.Pid, err = server.NewPid(s.Options.Pidfile); err != nil {
			s.Logger.Errorf("failed to write pidfile: %s", err)
			return err
		}
	}

	go func() {
		if err = http.ListenAndServe(s.Options.Address+":"+strconv.Itoa(s.Options.Port), handler); err != nil && err != http.ErrServerClosed {
			s.Logger.Errorf("unable to launch immugw: %+s", err)
		}
	}()
	<-s.quit
	return err
}

func (s *ImmuGwServer) Stop() error {
	s.Logger.Infof("stopping immugw: %v", s.Options)
	defer func() { s.quit <- struct{}{} }()
	return nil
}

func (s *ImmuGwServer) installShutdownHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer func() {
			s.quit <- struct{}{}
		}()
		<-c
		s.Logger.Infof("caught SIGTERM")
		if err := s.Stop(); err != nil {
			s.Logger.Errorf("shutdown error: %v", err)
		}
		s.Logger.Infof("shutdown completed")
	}()
}
