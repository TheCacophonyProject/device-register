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
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/TheCacophonyProject/event-reporter/v3/eventclient"
	"github.com/TheCacophonyProject/go-api"
	"github.com/TheCacophonyProject/go-config"
	"github.com/TheCacophonyProject/go-utils/logging"
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
var log = logging.NewLogger("info")

type Args struct {
	Reboot               bool   `arg:"-r,--reboot" help:"reboot device after registering"`
	API                  string `arg:"-a,--api" help:"url for the api server to register to"`
	RemoveDeviceConfig   bool   `arg:"-d,--remove-device-config" help:"remove the device config files. This is useful if you need to register as a new device or to a different server."`
	TestAPI              bool   `arg:"-t,--test-api" help:"use the test API. This will overwrite the API param"`
	Reregister           bool   `arg:"--reregister" help:"reregister the device to the same API with a new name and group"`
	Group                string `arg:"-g,--group" help:"new group name."`
	Name                 string `arg:"-n,--name" help:"new device name. If not given a random name will be generated"`
	Password             string `arg:"-p,--password" help:"new password. If not given a random password will be generated"`
	Prefix               string `arg:"--prefix" help:"prefix used in minion id"`
	RetryUntilRegistered bool   `arg:"--retry-until-registered" help:"will continue to try until it has registered"`
	logging.LogArgs
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
	err := runMain()
	if err != nil {
		log.Fatal(err)
	}
}

func runMain() error {
	args := procArgs()

	log.Printf("Running version: %s", version)

	cr := connrequester.NewConnectionRequester()
	log.Println("Requesting internet connection.")
	cr.Start()
	cr.WaitUntilUpLoop(connectionTimeout, connectionRetryInterval, -1)
	log.Println("Internet connection made.")
	defer cr.Stop()

	if args.Reregister {
		if err := reregister(args); err != nil {
			return err
		}
	} else {
		if isRegistered() {
			log.Println("Device is already registered, will not register again.")
			return nil
		}
		if args.RetryUntilRegistered {
			for !isRegistered() {
				if err := register(args); err != nil {
					log.Printf("Failed to register but will retry until registered. %v", err)
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
		log.Println("Restarting device.")
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

	saltId, err := checkMinionIDFile()
	if err != nil {
		return err
	}

	if args.RemoveDeviceConfig {
		if err := deleteDeviceConfigFiles(); err != nil {
			return err
		}
	}

	apiClient, err := api.Register(args.Name, args.Password, args.Group, apiString, saltId)
	if err != nil {
		return err
	}
	log.Println("Registered")
	log.Printf("deviceName: '%s', deviceID: '%d', API: '%s'", args.Name, apiClient.DeviceID(), apiString)

	if saltId == 0 {
		name := args.Prefix + "-" + strconv.Itoa(apiClient.DeviceID())
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

	log.Printf("Setting minion id to '%s'", name)
	if _, err := f.WriteString(name); err != nil {
		return err
	}
	return f.Sync()
}

func deleteDeviceConfigFiles() error {
	// Want to upload previous devices events first
	if err := eventclient.UploadEvents(); err != nil {
		return err
	}
	conf, err := config.New(config.DefaultConfigDir)
	if err != nil {
		return err
	}
	if err := conf.Unset(config.SecretsKey); err != nil {
		return err
	}
	return conf.Unset(config.DeviceKey)
}

func checkMinionIDFile() (int, error) {
	raw, err := os.ReadFile(minionIDFile)
	if os.IsNotExist(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	log.Println("Minion id file exists already, reading id file.")
	if len(raw) == 0 {
		log.Println("Minion id is empty. Will make new minion id.")
	} else {
		rawStr := string(raw)
		intStr := rawStr[strings.LastIndex(rawStr, "-")+1:]
		saltId, err := strconv.Atoi(intStr)
		if err != nil {
			log.Printf("Failed to extract salt ID from '%s'", string(raw))
			return 0, nil
		}
		log.Println("Minion ID:", saltId)
		return saltId, nil
	}
	return 0, nil
}

func reregister(args Args) error {
	// Want to upload previous devices events first
	if err := eventclient.UploadEvents(); err != nil {
		return err
	}

	apiClient, err := api.New()
	if err != nil {
		return err
	}
	log.Printf("Reregister with name '%s' and group '%s'", args.Name, args.Group)
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
