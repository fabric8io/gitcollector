//  Copyright 2016 Red Hat, Inc.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package watcher

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"
)

func TestNextNameName(t *testing.T) {
	emptyNames := []string{}
	names := []string{"foo", "bar", "whatnot"}

	assertNext(t, "", 0, "", 0, emptyNames)

	// check if item has been deleted
	assertNext(t, "doesNotExist", 0, "foo", 0, names)
	assertNext(t, "doesNotExist", 10, "foo", 0, names)

	// first
	assertNext(t, "", 0, "foo", 0, names)
	assertNext(t, "foo", 0, "bar", 1, names)
	assertNext(t, "bar", 1, "whatnot", 2, names)
	assertNext(t, "whatnot", 2, "foo", 0, names)
}

func assertNext(t *testing.T, current string, pos int, expectedName string, expectedPos int, names []string) {
	actualName, _ := nextName(current, pos, names)
	if actualName != expectedName {
		logErr(t, actualName, expectedName)
	}
}

func assertEquals(t *testing.T, found, expected string) {
	if found != expected {
		logErr(t, found, expected)
	}
}

func logErr(t *testing.T, found, expected string) {
	out := new(bytes.Buffer)

	_, _, line, ok := runtime.Caller(2)
	if ok {
		fmt.Fprintf(out, "Line: %d ", line)
	}
	fmt.Fprintf(out, "Unexpected response.\nExpecting to contain: \n %q\nGot:\n %q\n", expected, found)
	t.Errorf(out.String())
}
