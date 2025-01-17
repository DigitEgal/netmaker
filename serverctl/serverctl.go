package serverctl

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/servercfg"
)

func GetServerWGConf() (models.IntClient, error) {
	var server models.IntClient
	collection, err := database.FetchRecords(database.INT_CLIENTS_TABLE_NAME)
	if err != nil {
		return models.IntClient{}, errors.New("could not find comms server")
	}
	for _, value := range collection {
		json.Unmarshal([]byte(value), &server)
		if server.Network == "comms" && server.IsServer == "yes" {
			return server, nil
		}
	}
	return models.IntClient{}, errors.New("could not find comms server")
}

func InstallNetclient() error {
	if !FileExists("/etc/netclient/netclient") {
		_, err := copy("./netclient/netclient", "/etc/netclient/netclient")
		if err != nil {
			log.Println("could not create /etc/netclient")
			return err
		}
	}
	return nil
}

func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, errors.New(src + " is not a regular file")
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	err = os.Chmod(dst, 0755)
	if err != nil {
		log.Println(err)
	}
	return nBytes, err
}

func RemoveNetwork(network string) (bool, error) {
	_, err := os.Stat("/etc/netclient/netclient")
	if err != nil {
		log.Println("could not find /etc/netclient")
		return false, err
	}
	cmdoutput, err := local.RunCmd("/etc/netclient/netclient leave -n " + network)
	if err != nil {
		log.Println(string(cmdoutput))
		return false, err
	}
	log.Println("Server removed from network " + network)
	return true, err

}

func AddNetwork(network string) (bool, error) {
	pubip, err := servercfg.GetPublicIP()
	if err != nil {
		log.Println("could not get public IP.")
		return false, err
	}

	_, err = os.Stat("/etc/netclient")
	if os.IsNotExist(err) {
		os.Mkdir("/etc/netclient", 744)
	} else if err != nil {
		log.Println("could not find or create /etc/netclient")
		return false, err
	}
	token, err := functions.CreateServerToken(network)
	if err != nil {
		log.Println("could not create server token for " + network)
		return false, err
	}
	_, err = os.Stat("/etc/netclient/netclient")
	if os.IsNotExist(err) {
		err = InstallNetclient()
		if err != nil {
			return false, err
		}
	}
	err = os.Chmod("/etc/netclient/netclient", 0755)
	if err != nil {
		log.Println("could not change netclient directory permissions")
		return false, err
	}
	functions.PrintUserLog(models.NODE_SERVER_NAME,"executing network join: " + "/etc/netclient/netclient " + "join " + "-t " + token + " -name " + models.NODE_SERVER_NAME + " -endpoint " + pubip,0)

	joinCMD := exec.Command("/etc/netclient/netclient", "join", "-t", token, "-name", models.NODE_SERVER_NAME, "-endpoint", pubip)
	err = joinCMD.Start()
	
	
	if err != nil {
		log.Println(err)
	}
	log.Println("Waiting for join command to finish...")
	err = joinCMD.Wait()
	if err != nil {
		log.Println("Command finished with error: %v", err)
		return false, err
	}
	log.Println("Server added to network " + network)
	return true, err
}
