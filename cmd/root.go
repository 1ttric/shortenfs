package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "shortenfs",
		Short: "Shortenfs is a FUSE-based block device that stores data in someone else's URL shortener",
		Long: `Shortenfs implements a FUSE-based block device that writes its data into a user-configurable URL shortener.
You may then format the block device with the filesystem of your choice and mount it.`,
	}
)

func init() {
	rootCmd.AddCommand(mountCmd)
}

func Execute() {
	if err := mountCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
