package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/phamvinhdat/k8s-oidc-helper/internal/helper"
	_ "github.com/phamvinhdat/k8s-oidc-helper/internal/server"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	k8s_runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
)

const Version = "v1.0.0"

func main() {
	flag.BoolP("version", "v", false, "Print version and exit")
	flag.BoolP("open", "o", true, "Open the oauth approval URL in the browser")
	flag.String("client-id", "", "The ClientID for the application")
	flag.String("client-secret", "", "The ClientSecret for the application")
	flag.StringP("config", "c", "", "Path to a json file containing your application's ClientID and ClientSecret. Supercedes the --client-id and --client-secret flags.")
	flag.BoolP("write", "w", false, "Write config to file. Merges in the specified file")
	flag.String("file", "", "The file to write to. If not specified, `~/.kube/config` is used")

	_ = viper.BindPFlags(flag.CommandLine)
	viper.SetEnvPrefix("k8s-oidc-helper")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	flag.Parse()

	if viper.GetBool("version") {
		fmt.Printf("k8s-oidc-helper %s\n", Version)
		os.Exit(0)
	}

	var (
		gcf *helper.GoogleConfig
		err error
	)
	if configFile := viper.GetString("config"); len(viper.GetString("config")) > 0 {
		gcf, err = helper.ReadConfig(configFile)
		if err != nil {
			fmt.Printf("Error reading config file %s: %s\n", configFile, err)
			os.Exit(1)
		}
	}

	var clientID string
	var clientSecret string
	if gcf != nil {
		clientID = gcf.ClientID
		clientSecret = gcf.ClientSecret
	} else {
		clientID = viper.GetString("client-id")
		clientSecret = viper.GetString("client-secret")
	}

	oauthConfig := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8080",
		Scopes: []string{
			"openid",
			"email",
			"profile",
		},
	}

	authURL := oauthConfig.AuthCodeURL("", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	helper.LaunchBrowser(viper.GetBool("open"), authURL, clientID)
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the code Google gave you: ")
	code, _ := reader.ReadString('\n')
	code = strings.TrimSpace(code)

	tokResponse, err := oauthConfig.Exchange(context.Background(), code, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	if err != nil {
		fmt.Printf("Error getting tokens: %s\n", err)
		os.Exit(1)
	}

	rawIDToken, ok := tokResponse.Extra("id_token").(string)
	if !ok {
		log.Fatalln("missing id_token. require scope=openid")
	}

	email, err := helper.GetUserEmail(tokResponse.AccessToken)
	if err != nil {
		fmt.Printf("Error getting user email: %s\n", err)
		os.Exit(1)
	}

	authInfo := helper.GenerateAuthInfo(clientID, clientSecret, rawIDToken, tokResponse.RefreshToken)
	config := &clientcmdapi.Config{
		AuthInfos: map[string]*clientcmdapi.AuthInfo{email: authInfo},
	}

	if !viper.GetBool("write") {
		fmt.Println("\n# Add the following to your ~/.kube/config")

		json, err := k8s_runtime.Encode(clientcmdlatest.Codec, config)
		if err != nil {
			fmt.Printf("Unexpected error: %v", err)
			os.Exit(1)
		}
		output, err := yaml.JSONToYAML(json)
		if err != nil {
			fmt.Printf("Unexpected error: %v", err)
			os.Exit(1)
		}
		fmt.Printf("%v", string(output))
		return
	}

	tempKubeConfig, err := ioutil.TempFile("", "")
	if err != nil {
		fmt.Printf("Could not create tempfile: %v", err)
		os.Exit(1)
	}
	defer os.Remove(tempKubeConfig.Name())
	clientcmd.WriteToFile(*config, tempKubeConfig.Name())

	var kubeConfigPath string
	if viper.GetString("file") == "" {
		usr, err := user.Current()
		if err != nil {
			fmt.Printf("Could not determine current: %v", err)
			os.Exit(1)
		}
		kubeConfigPath = filepath.Join(usr.HomeDir, ".kube", "config")
	} else {
		kubeConfigPath = viper.GetString("file")
	}

	loadingRules := clientcmd.ClientConfigLoadingRules{
		Precedence: []string{tempKubeConfig.Name(), kubeConfigPath},
	}
	mergedConfig, err := loadingRules.Load()
	if err != nil {
		fmt.Printf("Could not merge configuration: %v", err)
		os.Exit(1)
	}

	clientcmd.WriteToFile(*mergedConfig, kubeConfigPath)
	fmt.Printf("Configuration has been written to %s\n", kubeConfigPath)
}
