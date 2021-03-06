/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/xact"
	"mynewt.apache.org/newt/util"
)

func logShowCmd(cmd *cobra.Command, args []string) {
	c := xact.NewLogShowCmd()
	c.SetTxOptions(nmutil.TxOptions())

	if len(args) >= 1 {
		c.Name = args[0]
	}
	if len(args) >= 2 {
		if args[1] == "last" {
			c.Index = 0
			c.Timestamp = -1
		} else {
			u64, err := strconv.ParseUint(args[1], 0, 64)
			if err != nil {
				nmUsage(cmd, err)
			}
			if u64 > 0xffffffff {
				nmUsage(cmd, util.NewNewtError("index out of range"))
			}
			c.Index = uint32(u64)
		}
	}
	if len(args) >= 3 && args[1] != "last" {
		var err error
		c.Timestamp, err = strconv.ParseInt(args[2], 0, 64)
		if err != nil {
			nmUsage(cmd, err)
		}
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.LogShowResult)
	fmt.Printf("Status: %d\n", sres.Status())
	fmt.Printf("Next index: %d\n", sres.Rsp.NextIndex)
	if len(sres.Rsp.Logs) == 0 {
		fmt.Printf("(no logs retrieved)\n")
		return
	}

	for _, log := range sres.Rsp.Logs {
		fmt.Printf("Name: %s\n", log.Name)
		fmt.Printf("Type: %s\n", nmp.LogTypeToString(log.Type))

		if len(log.Entries) == 0 {
			fmt.Printf("(no log entries retrieved)\n")
			return
		}

		fmt.Printf("%10s %22s | %11s %11s %s\n",
			"[index]", "[timestamp]", "[module]", "[level]", "[message]")
		for _, entry := range log.Entries {
			fmt.Printf("%10d %20dus | %10s: %10s: %s\n",
				entry.Index,
				entry.Timestamp,
				nmp.LogModuleToString(int(entry.Module)),
				nmp.LogLevelToString(int(entry.Level)),
				entry.Msg)
		}
	}
}

func logListCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewLogListCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.LogListResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("error: %d\n", sres.Rsp.Rc)
		return
	}

	sort.Strings(sres.Rsp.List)

	fmt.Printf("available logs:\n")
	for _, log := range sres.Rsp.List {
		fmt.Printf("    %s\n", log)
	}
}

func logModuleListCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewLogModuleListCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.LogModuleListResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("error: %d\n", sres.Rsp.Rc)
		return
	}

	names := make([]string, 0, len(sres.Rsp.Map))
	for k, _ := range sres.Rsp.Map {
		names = append(names, k)
	}
	sort.Strings(names)

	fmt.Printf("available modules:\n")
	for _, name := range names {
		fmt.Printf("    %s (%d)\n", name, sres.Rsp.Map[name])
	}
}

func logLevelListCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewLogLevelListCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.LogLevelListResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("error: %d\n", sres.Rsp.Rc)
		return
	}

	names := make([]string, 0, len(sres.Rsp.Map))
	for k, _ := range sres.Rsp.Map {
		names = append(names, k)
	}
	sort.Strings(names)

	fmt.Printf("available levels:\n")
	for _, name := range names {
		fmt.Printf("    %s: %d\n", name, sres.Rsp.Map[name])
	}
}

func logClearCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewLogClearCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.LogClearResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("error: %d\n", sres.Rsp.Rc)
		return
	}

	fmt.Printf("done\n")
}

func logCmd() *cobra.Command {
	logCmd := &cobra.Command{
		Use:   "log",
		Short: "Handles log on remote instance",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
	}

	showCmd := &cobra.Command{
		Use:   "show [log-name] [min-index] [min-timestamp]",
		Short: "Show log on target",
		Run:   logShowCmd,
	}
	logCmd.AddCommand(showCmd)

	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear log on target",
		Run:   logClearCmd,
	}
	logCmd.AddCommand(clearCmd)

	moduleListCmd := &cobra.Command{
		Use:   "module_list",
		Short: "Module List Command",
		Run:   logModuleListCmd,
	}
	logCmd.AddCommand(moduleListCmd)

	levelListCmd := &cobra.Command{
		Use:   "level_list",
		Short: "Level List Command",
		Run:   logLevelListCmd,
	}

	logCmd.AddCommand(levelListCmd)

	ListCmd := &cobra.Command{
		Use:   "list",
		Short: "Log List Command",
		Run:   logListCmd,
	}

	logCmd.AddCommand(ListCmd)
	return logCmd
}
