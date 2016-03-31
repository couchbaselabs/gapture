//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the
//  License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an "AS
//  IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
//  express or implied. See the License for the specific language
//  governing permissions and limitations under the License.

package main

import (
	"log"

	"github.com/couchbaselabs/gapture"
	"github.com/couchbaselabs/gapture/convert"
)

func main() {
	err := convert.ProcessDirs(
		[]string{"."},
		convert.Options{
			OnError: func(err error) { log.Println(err) },
			Logf:    log.Printf,
		})
	if err != nil {
		log.Fatal(err)
	}

	sampleStack()
}

func sampleStack() {
	gid, stack := gapture.Stack(0)
	log.Printf("gid: %d, stack: %s", gid, stack)
}

