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
	"github.com/TheCacophonyProject/go-config"
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
	defaultMinionIDPrefix   = "pi"
	retryWait               = 5 * time.Second
)

var version = "<not set>"

type Args struct {
	Reboot               bool   `arg:"-r,--reboot" help:"reboot device after registering"`
	API                  string `arg:"-a,--api" help:"url for the api server to register to"`
	IgnoreMinionID       bool   `arg:"-i,--ignore-minion-id" help:"don't check or write to minion id file"`
	RemoveDeviceConfig   bool   `arg:"-d,--remove-device-config" help:"remove the device config files. This is useful if you need to register as a new device or to a different server. This normally wants to be used with '-i'"`
	TestAPI              bool   `arg:"-t,--test-api" help:"use the test API. This will overwrite the API param"`
	Reregister           bool   `arg:"--reregister" help:"reregister the device to the same API with a new name and group"`
	Group                string `arg:"-g,--group" help:"new group name."`
	Name                 string `arg:"-n,--name" help:"new device name. If not given a random name will be generated"`
	Password             string `arg:"-p,--password" help:"new password. If not given a random password will be generated"`
	Prefix               string `arg:"--prefix" help:"prefix used in minion id"`
	RetryUntilRegistered bool   `arg:"--retry-until-registered" help:"will continue to try until it has registered"`
}

func (Args) Version() string {
	return version
}

func procArgs() Args {
	args := Args{
		API:    apiURL,
		Group:  defaultGroup,
		Prefix: defaultMinionIDPrefix,
	}

	arg.MustParse(&args)

	// Don't define these when args is declared so they are not shown as defaults in help
	if args.Name == "" {
		args.Name = petname.Generate(3, "-")
	}
	if args.Password == "" {
		args.Password = randString(20)
	}
	if args.TestAPI {
		args.API = testAPIURL
	}

	return args
}

func main() {
	rand.Seed(time.Now().UnixNano())
	err := runMain()
	if err != nil {
		log.Fatal(err)
	}
}

func runMain() error {
	log.SetFlags(0) // Removes default timestamp flag
	args := procArgs()
	log.Printf("running version: %s", version)

	cr := connrequester.NewConnectionRequester()
	log.Println("requesting internet connection")
	cr.Start()
	cr.WaitUntilUpLoop(connectionTimeout, connectionRetryInterval, -1)
	log.Println("internet connection made")
	defer cr.Stop()

	if args.Reregister {
		if err := reregister(args); err != nil {
			return err
		}
	} else {
		if args.RetryUntilRegistered {
			for !isRegistered() {
				if err := register(args); err != nil {
					log.Printf("failed to register but will retry until registered. %v", err)
					time.Sleep(retryWait)
				}
			}
		} else {
			if err := register(args); err != nil {
				return err
			}
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

func register(args Args) error {
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

	if args.RemoveDeviceConfig {
		if err := deleteDeviceConfigFiles(); err != nil {
			return err
		}
	}

	apiClient, err := api.Register(args.Name, args.Password, args.Group, apiString)
	if err != nil {
		return err
	}
	log.Println("registered")
	log.Printf("devicename: '%s', deviceID: '%d', API: '%s'", args.Name, apiClient.DeviceID(), apiString)

	if !args.IgnoreMinionID {
		name := args.Prefix + "-"
		if args.TestAPI {
			name += "test-"
		}
		name = name + strconv.Itoa(apiClient.DeviceID())
		if err := writeToMinionIDFile(name); err != nil {
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
	conf, err := config.New(config.DefaultConfigDir)
	if err != nil {
		return err
	}
	if err := conf.Unset(config.SecretsKey); err != nil {
		return err
	}
	return conf.Unset(config.DeviceKey)
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

func reregister(args Args) error {
	apiClient, err := api.New()
	if err != nil {
		return err
	}
	log.Printf("reregister with name '%s' and group '%s'", args.Name, args.Group)
	return apiClient.Reregister(args.Name, args.Group, args.Password)
}

func isRegistered() bool {
	configRW, err := config.New(config.DefaultConfigDir)
	if err != nil {
		log.Println(err)
		return false
	}
	var deviceConf config.Device
	if err := configRW.Unmarshal(config.DeviceKey, &deviceConf); err != nil {
		log.Println(err)
		return false
	}
	return deviceConf.ID != 0
}
