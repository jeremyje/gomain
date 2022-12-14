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

package gomain

import (
	"os"
	"path/filepath"
)

func exePath() string {
	return exePathFromPath(os.Args[0])
}

func exePathFromPath(prog string) string {
	absPath, err := filepath.Abs(prog)
	if err != nil {
		return prog
	}
	return absPath
}
