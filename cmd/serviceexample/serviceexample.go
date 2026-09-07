// Copyright 2022 Jeremy Edwards
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main is an example of how to use gomain.
package main

import (
	"context"
	"html"
	"log/slog"
	"net/http"
	"time"

	"github.com/cloudfra/gomain"
)

func main() {
	gomain.Run(appMain, gomain.Config{
		ServiceName:        "Service Example",
		ServiceDescription: "Service Example Service Description",
		Command:            "",
	})
}

func appMain(waitFunc func()) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
		safe := html.EscapeString(req.URL.Path)
		if _, err := resp.Write([]byte(safe)); err != nil {
			slog.With(err).Warn("Error writing HTTP response")
		}
	})
	s := &http.Server{
		Handler:           mux,
		Addr:              ":8181",
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		waitFunc()
		ctx := context.Background()
		slog.Info("Stopping server...")
		if err := s.Shutdown(ctx); err != nil {
			slog.With(err).Error("Error stopping server")
		}
	}()

	slog.Info("Serving on :8181")
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
