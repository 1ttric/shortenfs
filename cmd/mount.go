package cmd

import (
	"github.com/1ttric/shortenfs/internal"
	"github.com/1ttric/shortenfs/internal/config"
	"github.com/1ttric/shortenfs/internal/drivers"
	_ "github.com/1ttric/shortenfs/internal/drivers/bitly"
	_ "github.com/1ttric/shortenfs/internal/drivers/tinyurl"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	verbosity string
	cfgFile   string
	mountCmd  = &cobra.Command{
		Use:   "mount [mountpoint]",
		Short: "Mounts a block device running against the desired URL shortener at the given location",
		Args:  cobra.ExactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			driver, ok := drivers.Get(config.MainConfig.Driver)
			if !ok {
				log.Fatalf("unregistered driver %s", config.MainConfig.Driver)
			}
			// Decode implementation-specific driveropts into the driver struct itself
			err := mapstructure.Decode(config.MainConfig.DriverOpts, &driver)
			if err != nil {
				log.Fatalf("invalid driver options: %s", err.Error())
			}
			log.Debugf("mounting filesystem against %s", config.MainConfig.Driver)
			internal.Mount(args[0], driver)
			return nil
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	mountCmd.Flags().StringVarP(&cfgFile, "config", "c", "config.yml", "Specifies a shortener config file to read")
	mountCmd.Flags().StringVarP(&verbosity, "verbosity", "v", "info", "A Logrus verbosity level")
}

func initConfig() {
	verbosityLvl, err := log.ParseLevel(verbosity)
	if err != nil {
		log.Fatalf("could not parse loglevel: %s", err.Error())
	}
	log.SetLevel(verbosityLvl)
	if verbosityLvl >= log.DebugLevel {
		log.SetReportCaller(true)
		log.SetFormatter(&log.TextFormatter{
			DisableColors: true,
			FullTimestamp: true,
		})
	}
	config.Read(cfgFile)
}
