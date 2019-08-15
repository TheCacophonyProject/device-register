// device-register - helper for registering and renaming cacophony devices
//  Copyright (C) 2019, The Cacophony Project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"io/ioutil"
	"log"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/TheCacophonyProject/go-api"
	"github.com/TheCacophonyProject/modemd/connrequester"
	arg "github.com/alexflint/go-arg"
	petname "github.com/dustinkirkland/golang-petname"
)

const (
	connectionTimeout       = time.Minute * 2
	connectionRetryInterval = time.Minute * 10
	defaultGroup            = "new"
	apiURL                  = "https://api.cacophony.org.nz"
	testAPIURL              = "https://api-test.cacophony.org.nz"
	minionIDFile            = "/etc/salt/minion_id"
	deviceConfigFile        = "/etc/cacophony/device.yaml"
	devicePrivateConfigFile = "/etc/cacophony/device-priv.yaml"
	minionIDPrefix          = "pi-"
	minionIDTestPrefix      = "pi-test-"
)

var version = "<not set>"

type Args struct {
	Reboot             bool   `arg:"-r,--reboot" help:"reboot device after registering"`
	API                string `arg:"-a,--api" help:"url for the api server to register to"`
	IgnoreMinionID     bool   `arg:"-i,--ignore-minion-id" help:"don't check or write to minion id file"`
	RemoveDeviceConfig bool   `arg:"-d,--remove-device-Config" help:"remove the device config files. This is useful if you need to register as a new device or to a different server. This normally wants to be used with '-i'"`
	TestAPI            bool   `arg:"-t,--test-api" help:"use the test API. This will overwrite the API param"`
}

func (Args) Version() string {
	return version
}

func procArgs() Args {
	args := Args{
		API: apiURL,
	}
	arg.MustParse(&args)
	if args.TestAPI {
		args.API = testAPIURL
	}
	return args
}

func main() {
	err := runMain()
	if err != nil {
		log.Fatal(err)
	}
}

func runMain() error {
	log.SetFlags(0) // Removes default timestamp flag
	args := procArgs()
	log.Printf("running version: %s", version)

	apiURL, err := url.ParseRequestURI(args.API)
	if err != nil {
		return err
	}
	apiString := apiURL.String()

	if !args.IgnoreMinionID {
		if err := checkMinionIDFile(); err != nil {
			return err
		}
	}

	cr := connrequester.NewConnectionRequester()
	log.Println("requesting internet connection")
	cr.Start()
	cr.WaitUntilUpLoop(connectionTimeout, connectionRetryInterval, -1)
	log.Println("internet connection made")

	if args.RemoveDeviceConfig {
		if err := deleteDeviceConfigFiles(); err != nil {
			return err
		}
	}

	rand.Seed(time.Now().UnixNano())
	deviceName := petname.Generate(3, "-")
	apiClient, err := api.Register(deviceName, randString(20), defaultGroup, apiString)
	if err != nil {
		return err
	}
	cr.Stop()
	log.Println("registered")
	log.Printf("devicename: '%s', deviceID: '%d', API: '%s'", deviceName, apiClient.DeviceID(), apiString)

	if !args.IgnoreMinionID {
		var name string
		if args.TestAPI {
			name = minionIDTestPrefix
		} else {
			name = minionIDPrefix
		}
		name = name + strconv.Itoa(apiClient.DeviceID())
		if err := writeToMinionIDFile(name); err != nil {
			return err
		}
	}

	if args.Reboot {
		log.Println("restarting device")
		if err := exec.Command("reboot").Run(); err != nil {
			return err
		}
	}
	return nil
}

func writeToMinionIDFile(name string) error {
	f, err := os.Create(minionIDFile)
	if err != nil {
		return err
	}
	defer f.Close()

	log.Printf("setting minion id to '%s'", name)
	if _, err := f.WriteString(name); err != nil {
		return err
	}
	return nil
}

func deleteDeviceConfigFiles() error {
	if err := removeFileIfExist(deviceConfigFile); err != nil {
		return err
	}
	if err := removeFileIfExist(devicePrivateConfigFile); err != nil {
		return err
	}
	return nil
}

func removeFileIfExist(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func checkMinionIDFile() error {
	raw, err := ioutil.ReadFile(minionIDFile)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	log.Println("minion id file exists already, reading id file")
	if len(raw) == 0 {
		log.Println("minion id is empty. Will make new minion id")
	} else {
		log.Println("minion ID:", string(raw))
		log.Println("exiting as minion ID is already set")
		os.Exit(0)
	}
	return nil
}
