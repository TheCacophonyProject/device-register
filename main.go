// device-register - register the camera and save the config files
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
	apiURL                  = "http://192.168.178.20:1080"
	minionIDFile            = "/etc/salt/minion_id"
	minionIDPrefix          = "pi-"
)

var version = "<not set>"

type Args struct {
	Reboot bool   `arg:"-r,--reboot" help:"reboot device after registering"`
	API    string `arg:"-a,--api" help:"url for the api server to register to"`
}

func (Args) Version() string {
	return version
}

func procArgs() Args {
	args := Args{
		API: apiURL,
	}
	arg.MustParse(&args)
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
	log.Println(apiString)

	if err := checkMinionIDFile(); err != nil {
		return err
	}

	cr := connrequester.NewConnectionRequester()
	log.Println("requesting internet connection")
	cr.Start()
	cr.WaitUntilUpLoop(connectionTimeout, connectionRetryInterval, -1)
	log.Println("internet connection made")

	rand.Seed(time.Now().UnixNano())
	deviceName := petname.Generate(3, "-")
	apiClient, err := api.Register(deviceName, randString(20), defaultGroup, apiString)
	if err != nil {
		return err
	}
	cr.Stop()
	log.Println("registered")
	log.Printf("devicename: %s, deviceID: %d", deviceName, apiClient.DeviceID())

	f, err := os.Create(minionIDFile)
	if err != nil {
		return err
	}
	defer f.Close()

	name := minionIDPrefix + strconv.Itoa(apiClient.DeviceID())
	log.Printf("setting minion id to '%s'", name)
	if _, err := f.WriteString(name); err != nil {
		return err
	}

	if args.Reboot {
		log.Println("restarting device")
		if err := exec.Command("reboot").Run(); err != nil {
			return err
		}
	}
	return nil
}

func checkMinionIDFile() error {
	if _, err := os.Stat(minionIDFile); err == nil {
		log.Println("minion id file exists already, reading id file")
		raw, err := ioutil.ReadFile(minionIDFile)
		if err != nil {
			return err
		}

		if len(raw) == 0 {
			log.Println("minion id is empty. Will make new minion id")
		} else {
			log.Println("minion ID:", string(raw))
			log.Println("exiting as minion ID is already set")
			os.Exit(0)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}
