/*
Copyright 2017 Luis Pabón luis@portworx.com

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

//go:generate mockgen -package=driver -destination=driver.mock.go github.com/container-storage-interface/spec/lib/go/csi/v0 IdentityServer,ControllerServer,NodeServer

package driver

import (
	"net"
	"sync"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type CSIDriverServers struct {
	Controller csi.ControllerServer
	Identity   csi.IdentityServer
	Node       csi.NodeServer
}

type CSIDriver struct {
	listener net.Listener
	server   *grpc.Server
	servers  *CSIDriverServers
	wg       sync.WaitGroup
	running  bool
	lock     sync.Mutex
}

func NewCSIDriver(servers *CSIDriverServers) *CSIDriver {
	return &CSIDriver{
		servers: servers,
	}
}

func (c *CSIDriver) goServe(started chan<- bool) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		started <- true
		err := c.server.Serve(c.listener)
		if err != nil {
			panic(err.Error())
		}
	}()
}

func (c *CSIDriver) Address() string {
	return c.listener.Addr().String()
}
func (c *CSIDriver) Start(l net.Listener) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Set listener
	c.listener = l

	// Create a new grpc server
	c.server = grpc.NewServer()

	// Register Mock servers
	if c.servers.Controller != nil {
		csi.RegisterControllerServer(c.server, c.servers.Controller)
	}
	if c.servers.Identity != nil {
		csi.RegisterIdentityServer(c.server, c.servers.Identity)
	}
	if c.servers.Node != nil {
		csi.RegisterNodeServer(c.server, c.servers.Node)
	}
	reflection.Register(c.server)

	// Start listening for requests
	waitForServer := make(chan bool)
	c.goServe(waitForServer)
	<-waitForServer
	c.running = true
	return nil
}

func (c *CSIDriver) Stop() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.running {
		return
	}

	c.server.Stop()
	c.wg.Wait()
}

func (c *CSIDriver) Close() {
	c.server.Stop()
}

func (c *CSIDriver) IsRunning() bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.running
}
