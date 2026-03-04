package main

import (
	"fmt"
	"os"

	"github.com/dablon/kubesentinel/internal/config"
	"github.com/dablon/kubesentinel/internal/scanner"
	"github.com/spf13/cobra"
)

var version = "2.0.0"

var (
	cfgFile    string
	kubeconfig string
	namespace  string
	output     string
	severity   string
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

		if kubeconfig != "" {
			cfg.Kubeconfig = kubeconfig
		}

		s, err := scanner.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to connect to cluster: %w", err)
		}

		results, err := s.Scan(namespace, severity)
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		switch output {
		case "json":
			if err := s.PrintJSON(results); err != nil {
				return fmt.Errorf("failed to write JSON: %w", err)
			}
		case "table":
			s.PrintTable(results)
		default:
			s.PrintResults(results)
		}

		// Graduated exit codes
		switch {
		case results.Summary.Critical > 0:
			os.Exit(1)
		case results.Summary.High > 0:
			os.Exit(2)
		case results.Summary.Medium > 0:
			os.Exit(3)
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
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "namespace to scan")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "text", "output format (text/json/table)")
	rootCmd.PersistentFlags().StringVarP(&severity, "severity", "s", "all", "severity filter (all, critical, high, medium, low, or high+ for high and above)")
	scanCmd.Flags().BoolVarP(&scanner.Verbose, "verbose", "v", false, "verbose output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
