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
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mssola/dockerclient"
)

func TestMockClient(t *testing.T) {
	dockerClient = &mockClient{}

	client := getDockerClient()
	to := reflect.TypeOf(client)
	if to.String() != "*main.mockClient" {
		t.Fatal("Wrong type for the client")
	}

	iface := reflect.TypeOf((*DockerClient)(nil)).Elem()
	if !to.Implements(iface) {
		t.Fatal("The mock type does not implement the DockerClient interface!")
	}
}

// This is the only test that will check for the actual real connection, so for
// safety reasons we do `dockerClient = nil` before and after the actual test.
func TestDockerClient(t *testing.T) {
	dockerClient = nil

	// This test will work even if docker is not running. Take a look at the
	// implementation of it for more details.
	client := getDockerClient()

	docker, ok := client.(*dockerclient.DockerClient)
	if !ok {
		t.Fatal("Could not cast to dockerclient.DockerClient")
	}

	if docker.URL.Scheme != "http" {
		t.Fatalf("Unexpected scheme: %v\n", docker.URL.Scheme)
	}
	if docker.URL.Host != "unix.sock" {
		t.Fatalf("Unexpected host: %v\n", docker.URL.Host)
	}

	dockerClient = nil
}

func TestRunCommandInContainerCreateFailure(t *testing.T) {
	dockerClient = &mockClient{createFail: true}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	if _, err := runCommandInContainer("fail", []string{}, false); err == nil {
		t.Fatal("It should've failed\n")
	}
	if !strings.Contains(buffer.String(), "Create failed") {
		t.Fatal("It should've logged something expected\n")
	}
}

func TestRunCommandInContainerStartFailure(t *testing.T) {
	dockerClient = &mockClient{startFail: true}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	if ret := checkCommandInImage("fail", ""); ret {
		t.Fatal("It should've failed\n")
	}

	// The only logged stuff is that the created container has been removed.
	lines := strings.Split(buffer.String(), "\n")
	if len(lines) != 3 {
		t.Fatal("Wrong number of lines")
	}
	if !strings.Contains(buffer.String(), "Removed container") {
		t.Fatal("It should've logged something expected\n")
	}
	if !strings.Contains(buffer.String(), "Start failed") {
		t.Fatal("It should've logged something expected\n")
	}
}

func TestRunCommandInContainerContainerLogsFailure(t *testing.T) {
	dockerClient = &mockClient{logFail: true}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	_, err := runCommandInContainer("opensuse", []string{"zypper"}, true)
	if err == nil {
		t.Fatal("It should have failed\n")
	}

	if err.Error() != "Fake log failure" {
		t.Fatal("Should have failed because of a logging issue")
	}
}

func TestRunCommandInContainerStreaming(t *testing.T) {
	mock := mockClient{}
	dockerClient = &mock

	temp, err := ioutil.TempFile("", "zypper_docker")
	if err != nil {
		t.Fatal("Could not setup test")
	}

	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()

	original := os.Stdout
	os.Stdout = temp

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	_, err = runCommandInContainer("opensuse", []string{"foo"}, true)

	// restore stdout
	os.Stdout = original

	if err != nil {
		t.Fatal("It shouldn't have failed\n")
	}

	b, err := ioutil.ReadFile(temp.Name())
	if err != nil {
		t.Fatal("Could not read temporary file")
	}

	if !strings.Contains(string(b), "streaming buffer initialized") {
		t.Fatal("The streaming buffer should have been initialized\n")
	}
}

func TestRunCommandInContainerCommandFailure(t *testing.T) {
	dockerClient = &mockClient{commandFail: true}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	_, err := runCommandInContainer("busybox", []string{"zypper"}, false)
	if err == nil {
		t.Fatal("It should've failed\n")
	}

	if err.Error() != "Command exited with status 1" {
		t.Fatal("Wrong type of error received")
	}
}

func TestCheckCommandInImageWaitFailed(t *testing.T) {
	dockerClient = &mockClient{
		waitFail:  true,
		waitSleep: 100 * time.Millisecond,
	}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	if res := checkCommandInImage("fail", ""); res {
		t.Fatal("It should've failed\n")
	}

	lines := strings.Split(buffer.String(), "\n")
	if len(lines) != 3 {
		t.Fatal("Wrong number of lines")
	}
	if !strings.Contains(buffer.String(), "Wait failed") {
		t.Fatal("It should've logged something expected\n")
	}
	if !strings.Contains(buffer.String(), "Removed container zypper-docker-private-fail") {
		t.Fatal("It should've logged something expected\n")
	}
}

func TestCheckCommandInImageWaitTimedOut(t *testing.T) {
	dockerClient = &mockClient{waitSleep: containerTimeout * 2}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	if res := checkCommandInImage("fail", ""); res {
		t.Fatal("It should've failed\n")
	}

	lines := strings.Split(buffer.String(), "\n")
	if len(lines) != 4 {
		t.Fatal("Wrong number of lines")
	}
	if !strings.Contains(buffer.String(), "Timed out when waiting for a container.") {
		t.Fatal("It should've logged something expected\n")
	}
	if !strings.Contains(buffer.String(), "Removed container zypper-docker-private-fail") {
		t.Fatal("It should've logged something expected\n")
	}
}

func TestCheckCommandInImageSuccess(t *testing.T) {
	dockerClient = &mockClient{waitSleep: 100 * time.Millisecond}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	if res := checkCommandInImage("ok", ""); !res {
		t.Fatal("It should've been ok\n")
	}

	lines := strings.Split(buffer.String(), "\n")
	if len(lines) != 2 {
		t.Fatal("Wrong number of lines")
	}
	if !strings.Contains(buffer.String(), "Removed container zypper-docker-private-ok") {
		t.Fatal("It should've logged something expected\n")
	}
}

func TestRemoveContainerFail(t *testing.T) {
	dockerClient = &mockClient{removeFail: true}

	buffer := bytes.NewBuffer([]byte{})
	log.SetOutput(buffer)
	removeContainer("fail")
	if !strings.Contains(buffer.String(), "Remove failed") {
		t.Fatal("It should've logged something expected\n")
	}

	// Making sure that the logger has not print the "success" message
	// from the mock type.
	lines := strings.Split(buffer.String(), "\n")
	if len(lines) != 2 {
		t.Fatal("Wrong number of lines")
	}
}