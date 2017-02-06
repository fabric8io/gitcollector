/**
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package util

import (
	"fmt"
	"os"

	"github.com/daviddengcn/go-colortext"
)

func Infof(msg string, args ...interface{}) {
	Info(fmt.Sprintf(msg, args...))
}

func Info(msg string) {
	fmt.Print(msg)
}

func Blank() {
	fmt.Println()
}

func Warnf(msg string, args ...interface{}) {
	Warn(fmt.Sprintf(msg, args...))
}

func Warn(msg string) {
	ct.ChangeColor(ct.Yellow, false, ct.None, false)
	fmt.Print(msg)
	ct.ResetColor()
}

func Errorf(msg string, args ...interface{}) {
	Error(fmt.Sprintf(msg, args...))
}

func Error(msg string) {
	ct.ChangeColor(ct.Red, true, ct.None, false)
	fmt.Print(msg)
	ct.ResetColor()
}

func Fatalf(msg string, args ...interface{}) {
	Fatal(fmt.Sprintf(msg, args...))
}

func Fatal(msg string) {
	ct.ChangeColor(ct.Red, true, ct.None, false)
	fmt.Print(msg)
	ct.ResetColor()
	os.Exit(1)
}

func Success(msg string) {
	ct.ChangeColor(ct.Green, false, ct.None, false)
	fmt.Print(msg)
	ct.ResetColor()
}

func Successf(msg string, args ...interface{}) {
	Success(fmt.Sprintf(msg, args...))
}

func Failure(msg string) {
	ct.ChangeColor(ct.Red, false, ct.None, false)
	fmt.Print(msg)
	ct.ResetColor()
}

func Failuref(msg string, args ...interface{}) {
	Failure(fmt.Sprintf(msg, args...))
}

// AskForConfirmation uses Scanln to parse user input. A user must type in "yes" or "no" and
// then press enter. It has fuzzy matching, so "y", "Y", "yes", "YES", and "Yes" all count as
// confirmations. If the input is not recognized, it will ask again. The function does not return
// until it gets a valid response from the user. Typically, you should use fmt to print out a question
// before calling askForConfirmation. E.g. fmt.Println("WARNING: Are you sure? (yes/no)")
func AskForConfirmation(def bool) bool {
	var response string
	fmt.Scanln(&response)
	if len(response) == 0 {
		return def
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if containsString(okayResponses, response) {
		return true
	} else if containsString(nokayResponses, response) {
		return false
	} else {
		Warn("Please type y or n & press enter: ")
		return AskForConfirmation(def)
	}
}

// You might want to put the following two functions in a separate utility package.

// posString returns the first index of element in slice.
// If slice does not contain element, returns -1.
func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}

// containsString returns true iff slice contains element
func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}
