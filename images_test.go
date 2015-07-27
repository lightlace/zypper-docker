// Copyright (c) 2015 SUSE LLC. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codegangsta/cli"
)

func testContext(force bool) *cli.Context {
	set := flag.NewFlagSet("test", 0)
	set.Bool("force", force, "doc")
	return cli.NewContext(nil, set, nil)
}

func TestImagesCmdFail(t *testing.T) {
	dockerClient = &mockClient{listFail: true}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	imagesCmd(testContext(false))

	lines := strings.Split(buffer.String(), "\n")
	if len(lines) != 2 {
		t.Fatal("Wrong number of lines")
	}
	if !strings.Contains(buffer.String(), "List Failed") {
		t.Fatal("It should've logged something expected\n")
	}
}

func TestImagesListEmpty(t *testing.T) {
	dockerClient = &mockClient{listEmpty: true}

	temp, err := ioutil.TempFile("", "zypper")
	if err != nil {
		t.Fatal("Could not setup test")
	}
	original := os.Stdout
	os.Stdout = temp

	imagesCmd(testContext(false))
	b, err := ioutil.ReadFile(temp.Name())
	if err != nil {
		t.Fatal("Could not read temporary file")
	}

	_ = temp.Close()
	_ = os.Remove(temp.Name())
	os.Stdout = original

	lines := strings.Split(string(b), "\n")
	if len(lines) != 3 {
		t.Fatal("Wrong number of lines")
	}
	if !strings.HasPrefix(lines[1], "REPOSITORY") {
		t.Fatal("Wrong contents")
	}
}

func TestImagesListOk(t *testing.T) {
	dockerClient = &mockClient{waitSleep: 100 * time.Millisecond}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	temp, err := ioutil.TempFile("", "zypper")
	if err != nil {
		t.Fatal("Could not setup test")
	}
	original := os.Stdout
	os.Stdout = temp

	imagesCmd(testContext(false))
	b, err := ioutil.ReadFile(temp.Name())
	if err != nil {
		os.Stdout = original
		t.Fatal("Could not read temporary file")
	}

	_ = temp.Close()
	_ = os.Remove(temp.Name())
	os.Stdout = original

	lines := strings.Split(string(b), "\n")
	if len(lines) != 5 {
		t.Fatal("Wrong number of lines")
	}
	if !strings.HasPrefix(lines[1], "REPOSITORY") {
		t.Fatal("Wrong contents")
	}
	str := "opensuse            latest              1                   Less than a second ago   254.5 MB"
	if lines[2] != str {
		t.Fatal("Wrong contents")
	}
	str = "opensuse            13.2                2                   Less than a second ago   254.5 MB"
	if lines[3] != str {
		t.Fatal("Wrong contents")
	}
}

func TestImagesForce(t *testing.T) {
	dockerClient = &mockClient{waitSleep: 100 * time.Millisecond}

	temp, err := ioutil.TempFile("", "zypper")
	if err != nil {
		t.Fatal("Could not setup test")
	}
	original := os.Stdout
	os.Stdout = temp

	cache := os.Getenv("XDG_CACHE_HOME")
	abs, _ := filepath.Abs(".")
	test := filepath.Join(abs, "test")

	defer func() {
		_ = os.Setenv("XDG_CACHE_HOME", cache)
		_ = os.Remove(filepath.Join(test, cacheName))
	}()
	_ = os.Setenv("XDG_CACHE_HOME", test)

	// Dump some dummy value.
	cd := getCacheFile()
	cd.Suse = []string{"1234"}
	cd.flush()

	// Check that they are really written there.
	cd = getCacheFile()
	if len(cd.Suse) != 1 || cd.Suse[0] != "1234" {
		t.Fatal("Unexpected value")
	}

	// Luke, use the force!
	imagesCmd(testContext(true))
	cd = getCacheFile()

	if !cd.Valid {
		t.Fatal("It should be valid")
	}
	for i, v := range []string{"1", "2", "4"} {
		if cd.Suse[i] != v {
			t.Fatal("Unexpected value")
		}
	}
	if len(cd.Other) != 1 || cd.Other[0] != "3" {
		t.Fatal("Unexpected value")
	}

	_ = temp.Close()
	_ = os.Remove(temp.Name())
	os.Stdout = original
}