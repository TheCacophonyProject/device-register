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
	"errors"
	"fmt"
	"log"

	"github.com/TheCacophonyProject/go-api"
	"github.com/alexflint/go-arg"
)

var version = "<not set>"

type Args struct {
	Name  string `arg:"-n,--name" help:"new devicename"`
	Group string `arg:"-g,--group" help:"new groupname"`
}

func (Args) Version() string {
	return version
}

func procArgs() Args {
	args := Args{}
	arg.MustParse(&args)
	return args
}

func main() {
	if err := runMain(); err != nil {
		log.Fatal(err)
	}
}

func runMain() error {
	log.SetFlags(0) // Removes default timestamp flag
	args := procArgs()
	log.Printf("running version: %s", version)

	if args.Group == "" && args.Name == "" {
		return errors.New("new group or new device must be set")
	}

	apiClient, err := api.New()
	if err != nil {
		return err
	}

	var name string
	if args.Name == "" {
		name = apiClient.DeviceName()
	} else {
		name = args.Name
	}

	var group string
	if args.Group == "" {
		group = apiClient.GroupName()
	} else {
		group = args.Group
	}

	log.Printf("setting name to '%s' and group to '%s'", name, group)

	if err := apiClient.Rename(name, group); err != nil {
		fmt.Printf(err.Error())
		return err
	}
	return nil
}
