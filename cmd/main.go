package main

import (
	"fmt"
	"os"
	"github.com/dablon/kubesentinel/internal/scanner"
	"github.com/dablon/kubesentinel/internal/config"
	"github.com/spf13/cobra"
)

var version = "1.0.0"

var (
	cfgFile   string
	namespace string
	output   string
	severity string
)

var rootCmd = &cobra.Command{
	Use:     "kubesentinel",
	Short:   "Kubernetes Security Scanner",
	Long:    "KubeSentinel scans Kubernetes clusters for security vulnerabilities and misconfigurations.",
	Version: version,
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan Kubernetes cluster for security issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		s := scanner.New(cfg)
		results, err := s.Scan(namespace, severity)
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		if output == "json" {
			s.PrintJSON(results)
		} else {
			s.PrintResults(results)
		}

		if results.HasCritical() {
			os.Exit(1)
		}
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "namespace to scan")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "text", "output format (text/json)")
	rootCmd.PersistentFlags().StringVarP(&severity, "severity", "s", "all", "severity filter")
	scanCmd.Flags().BoolVarP(&scanner.Verbose, "verbose", "v", false, "verbose output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
