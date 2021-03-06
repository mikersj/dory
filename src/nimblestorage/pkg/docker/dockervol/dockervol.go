/*
(c) Copyright 2017 Hewlett Packard Enterprise Development LP

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

package dockervol

import (
	"fmt"
	"nimblestorage/pkg/connectivity"
	"nimblestorage/pkg/util"
	"strings"
)

const (
	//ActivateURI is /Plugin.Activate
	ActivateURI = "/Plugin.Activate"
	//CreateURI is /VolumeDriver.Create
	CreateURI = "/VolumeDriver.Create"
	//ListURI is /VolumeDriver.List
	ListURI = "/VolumeDriver.List"
	//CapabilitiesURI is /VolumeDriver.Capabilities
	CapabilitiesURI = "/VolumeDriver.Capabilities"
	//RemoveURI is /VolumeDriver.Remove
	RemoveURI = "/VolumeDriver.Remove"
	//MountURI is /VolumeDriver.Mount
	MountURI = "/VolumeDriver.Mount"
	//UnmountURI is /VolumeDriver.Unmount
	UnmountURI = "/VolumeDriver.Unmount"
	//GetURI is /VolumeDriver.Get
	GetURI = "/VolumeDriver.Get"
	//NotFound describes the beginning of the not found error message
	NotFound = "Unable to find"
)

var (
	//SocketPath is the full path to the location of the socket file for the nimble volume plugin
	SocketPath = "/run/docker/plugins/nimble.sock"
	//StripK8sFromOptions indicates if k8s namespace should be stripped fromoptions
	StripK8sFromOptions = true
)

//Errorer describes the ability get the embedded error
type Errorer interface {
	getErr() string
}

//Request is the basic request to use when talking to the driver
type Request struct {
	Name string                 `json:"Name,omitempty"`
	Opts map[string]interface{} `json:"Opts,omitempty"`
}

//MountRequest is used to mount and unmount volumes
type MountRequest struct {
	Name string `json:"Name,omitempty"`
	ID   string `json:"ID,omitempty"`
}

//MountResponse is returned from the volume driver
type MountResponse struct {
	Mountpoint string `json:"Mountpoint,omitempty"`
	Err        string `json:"Err,omitempty"`
}

func (g *MountResponse) getErr() string {
	return g.Err
}

//GetResponse is returned from the volume driver
type GetResponse struct {
	Volume DockerVolume `json:"Volume,omitempty"`
	Err    string       `json:"Err,omitempty"`
}

func (g *GetResponse) getErr() string {
	return g.Err
}

//DockerVolume represents the details about a docker volume
type DockerVolume struct {
	Name       string                 `json:"Name,omitempty"`
	Mountpoint string                 `json:"Mountpoint,omitempty"`
	Status     map[string]interface{} `json:"Status,omitempty"`
}

//Get a docker volume by docker name returning the response from the driver
func Get(name string) (*GetResponse, error) {
	var req = &Request{Name: name}
	var res = &GetResponse{}

	err := driverRun(&connectivity.Request{
		Action:        "POST",
		Path:          GetURI,
		Payload:       req,
		Response:      res,
		ResponseError: res})
	if err != nil {
		util.LogInfo.Printf("unable to get docker volume using %s - %s\n", name, err.Error())
		return nil, err
	}

	if err = driverErrorCheck(res); err != nil {
		util.LogInfo.Printf("unable to get docker volume using %s - %s\n", name, err.Error())
		return nil, err
	}
	util.LogDebug.Printf("returning %#v", res)
	return res, nil
}

//Create a docker volume returning the docker volume name
func Create(name string, options map[string]interface{}) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	for key := range options {
		if key == "name" || (StripK8sFromOptions && strings.HasPrefix(key, "kubernetes.io")) {
			delete(options, key)
		}
	}
	var req = &Request{Name: name, Opts: options}
	var res = &GetResponse{}

	err := driverRun(&connectivity.Request{
		Action:        "POST",
		Path:          CreateURI,
		Payload:       req,
		Response:      res,
		ResponseError: res})
	if err != nil {
		util.LogError.Printf("unable to create docker volume using %v & %v - %s\n", name, options, err.Error())
		return "", err
	}

	if err = driverErrorCheck(res); err != nil {
		util.LogError.Printf("unable to create docker volume using %v & %v - %s\n", name, options, err.Error())
		return "", err
	}

	return res.Volume.Name, nil
}

//Mount attaches and mounts a nimble volume returning the path
func Mount(name, mountID string) (string, error) {
	m, err := mounter(name, mountID, MountURI)
	if err != nil {
		return "", err
	}
	return m, nil
}

//Unmount unmounts and detaches nimble volume
func Unmount(name, mountID string) error {
	_, err := mounter(name, mountID, UnmountURI)
	if err != nil {
		return err
	}
	return nil
}

func mounter(name, mountID string, path string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	var req = &MountRequest{Name: name, ID: mountID}
	var res = &MountResponse{}

	err := driverRun(&connectivity.Request{
		Action:        "POST",
		Path:          path,
		Payload:       req,
		Response:      res,
		ResponseError: res})
	if err != nil {
		util.LogError.Printf("%s failed %v & %v - %s\n", path, name, mountID, err.Error())
		return "", err
	}

	if err = driverErrorCheck(res); err != nil {
		util.LogError.Printf("%s failed %v & %v - %s\n", path, name, mountID, err.Error())
		return "", err
	}

	return res.Mountpoint, nil
}

func newDockerVolumeClient() *connectivity.Client {
	return connectivity.NewSocketClient(SocketPath)
}

func driverRun(r *connectivity.Request) error {
	return newDockerVolumeClient().DoJSON(r)
}

func driverErrorCheck(e Errorer) error {
	if e.getErr() != "" {
		return fmt.Errorf(e.getErr())
	}
	return nil
}
