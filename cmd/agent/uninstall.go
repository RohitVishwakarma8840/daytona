// Copyright 2024 Daytona Platforms Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd_agent

import (
	"fmt"
	"strconv"
	"time"

	"github.com/daytonaio/daytona/config"
	"github.com/daytonaio/daytona/remote_installer"

	cmd_profile "github.com/daytonaio/daytona/cmd/profile"
	profile_list "github.com/daytonaio/daytona/cmd/views/profile_list"
	views_util "github.com/daytonaio/daytona/cmd/views/util"

	"github.com/briandowns/spinner"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the Daytona Agent from a remote host",
	Long:  "Uninstall the Daytona Agent from a remote host. Note: this command will not uninstall Docker from your system",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)

		c, err := config.GetConfig()
		if err != nil {
			log.Fatal(err)
		}

		profilesList := c.Profiles
		chosenProfileId := profile_list.GetProfileIdFromPrompt(profilesList, c.ActiveProfileId, "Choose a profile to uninstall from", true)

		if chosenProfileId == profile_list.NewProfileId {
			chosenProfileId = cmd_profile.CreateProfile(c, false)
		}

		chosenProfile, err := c.GetProfile(chosenProfileId)
		if err != nil {
			log.Fatal(err)
		}

		views_util.RenderMainTitle("REMOTE UNINSTALLER")

		var client *ssh.Client

		sshConfig := GetSshConfigFromProfile(&chosenProfile)

		fmt.Println("Connecting to remote host ...")
		s.Start()
		defer s.Stop()

		client, err = ssh.Dial("tcp", chosenProfile.Hostname+":"+strconv.Itoa(chosenProfile.Port), sshConfig)
		if err != nil {
			log.Fatal(err)
		}

		installer := &remote_installer.RemoteInstaller{
			Client:     client,
			BinaryUrl:  config.GetBinaryUrls(),
			Downloader: remote_installer.DownloaderCurl,
		}

		s.Stop()

		remoteOs, err := installer.DetectOs()
		if err != nil {
			log.Error("Failed to detect remote OS")
			log.Fatal(err)
		}

		agentRegistered, err := installer.AgentRegistered()
		if err != nil {
			log.Fatal(err)
		}

		if !agentRegistered {
			fmt.Println("Daytona Agent is not installed on the remote machine.\n")
			return
		}

		sudoPasswordRequired, err := installer.SudoPasswordRequired()
		if err != nil {
			log.Fatal(err)
		}

		var sessionPassword string

		if sudoPasswordRequired && (chosenProfile.Auth.Password == nil || *chosenProfile.Auth.Password == "") {
			if chosenProfile.Auth.Password == nil || *chosenProfile.Auth.Password == "" {
				fmt.Printf("Enter password for user %s:", chosenProfile.Auth.User)
				password, err := term.ReadPassword(0)
				fmt.Println()
				if err != nil {
					log.Fatal(err)
				}
				sessionPassword = string(password)
			} else {
				sessionPassword = *chosenProfile.Auth.Password
			}
		}
		installer.Password = sessionPassword

		err = installer.RemoveDaemon(*remoteOs)
		if err != nil {
			log.Error("Failed to remove Daytona daemon")
			log.Fatal(err)
		}

		err = installer.RemoveBinary(*remoteOs)
		if err != nil {
			log.Error("Failed to remove Daytona binary")
			log.Fatal(err)
		}

		fmt.Println("\nDaytona Agent has been successfully uninstalled.\n")
	},
}