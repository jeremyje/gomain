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

// Package testing provides a helper function for testing gomain-based applications.
package testing

import (
	"github.com/cloudfra/gomain"
	"github.com/cloudfra/gomain/internal"
)

// Main is a helper function for testing gomain-based applications. It runs the provided main function in a separate goroutine and returns a function that can be called to stop the application and wait for it to exit.
func Main(f gomain.MainFunc) func() error {
	errCh := make(chan error, 1)
	runCtx := internal.NewRunCtx()
	go func() {
		errCh <- f(runCtx.Wait)
		close(errCh)
	}()

	return func() error {
		runCtx.Kill()
		err := <-errCh
		return err
	}
}
