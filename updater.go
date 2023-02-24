package main

import (
	"crypto/aes"
	"encoding/json"
	"errors"
	"fmt"
	cn "github.com/ilinovalex86/connection"
	ex "github.com/ilinovalex86/explorer"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const configFile = "conf.txt"
const key = "2112751343910010"

var conf config
var clientApp = "client"
var newClientApp = "newClient"
var command = "./" + clientApp

type config struct {
	UpdaterServer string
	TcpServer     string
	StreamServer  string
	VersionClient string
	ClientId      string
}

type clientData struct {
	Sep      string
	BasePath string
	conn     net.Conn
	Version  string
	System   string
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	if ex.ExistFile(configFile) {
		data, err := ex.ReadFileFull(configFile)
		check(err)
		err = json.Unmarshal(data, &conf)
		check(err)
	} else {
		conf := config{
			UpdaterServer: "127.0.0.1:50000",
			TcpServer:     "127.0.0.1:50001",
			StreamServer:  "127.0.0.1:50002",
			VersionClient: "0.0.0",
			ClientId:      "----------------",
		}
		data, err := json.MarshalIndent(&conf, "", "  ")
		check(err)
		err = ioutil.WriteFile(configFile, data, 0644)
		check(err)
		log.Fatal("Файл конфигурации не найден. Создан новый файл конфигурации.")
	}
	if runtime.GOOS == "windows" {
		clientApp += ".exe"
		newClientApp += ".exe"
		path, err := os.Executable()
		check(err)
		i := strings.LastIndex(path, "\\")
		path = strings.Replace(path, path[i+1:], "", 1)
		clientApp = path + clientApp
		command = clientApp
		newClientApp = path + newClientApp
	}
	path, err := os.Stat(clientApp)
	if err != nil || path.IsDir() {
		log.Fatal("no client")
	}
}

func newClient() *clientData {
	fmt.Println("Инициализация")
	cl := &clientData{
		Sep:      ex.Sep,
		BasePath: ex.BasePath,
		Version:  conf.VersionClient,
		System:   ex.System,
	}
	fmt.Printf("BasePath: %12s \n", cl.BasePath)
	fmt.Printf("id: %8s \n", conf.ClientId)
	fmt.Printf("Version: %8s \n", conf.VersionClient)
	return cl
}

//Получает файл актуального клиента
func (cl *clientData) downloadNewClient(q cn.Query) error {
	var err error
	cn.SendSync(cl.conn)
	err = cn.GetFile(q.Query, q.DataLen, cl.conn)
	if err != nil {
		return errors.New("downloadNewClient")
	}
	return nil
}

//Подключается к серверу и получаетот него указание
func (cl *clientData) connect() error {
	if !cl.validOnServer(cl.conn) {
		log.Fatal("Valid on Server")
	}
	err := cn.SendString(conf.ClientId, cl.conn)
	cn.ReadSync(cl.conn)
	jsonData, err := json.Marshal(cl)
	err = cn.SendBytesWithDelim(jsonData, cl.conn)
	if err != nil {
		return err
	}
	q, err := cn.ReadQuery(cl.conn)
	if err != nil {
		return err
	}
	switch q.Method {
	case "already exist":
		log.Fatal("already exist")
	case "downloadNewClient":
		err = cl.downloadNewClient(q)
		if err != nil {
			return err
		}
		path, err := os.Stat(newClientApp)
		if err == nil && !path.IsDir() {
			fmt.Println("update client")
			err = os.Remove(clientApp)
			check(err)
			err := os.Rename(newClientApp, clientApp)
			check(err)
			if runtime.GOOS != "windows" {
				time.Sleep(time.Second)
				err = os.Chmod(clientApp, 0777)
				check(err)
				time.Sleep(time.Second)
			}
		} else {
			log.Fatal("newClientApp: if err == nil && !path.IsDir()")
		}
		return nil
	case "lenClient":
		data, err := ex.ReadFileFull(clientApp)
		check(err)
		if len(data) != q.DataLen {
			return errors.New("wrong client")
		}
		return nil
	}
	return nil
}

//Проходит валидацию при подключении к серверу
func (cl *clientData) validOnServer(conn net.Conn) bool {
	var code = make([]byte, 16)
	bc, err := aes.NewCipher([]byte(key))
	err = cn.SendString("updater", conn)
	if err != nil {
		return false
	}
	data, err := cn.ReadBytesByLen(16, conn)
	if err != nil {
		return false
	}
	bc.Decrypt(code, data)
	s := string(code)
	res := s[len(s)/2:] + s[:len(s)/2]
	bc.Encrypt(code, []byte(res))
	err = cn.SendBytes(code, conn)
	if err != nil {
		return false
	}
	mes, err := cn.ReadString(conn)
	if err != nil || mes != "ok" {
		return false
	}
	return true
}

//Запускает клиента
func clientRun() int {
	cmd := exec.Command(command)
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	procState := cmd.ProcessState
	return procState.ExitCode()
}

func main() {
	cl := newClient()
	fmt.Println("Start updater")
	for {
		conn, err := net.Dial("tcp", conf.UpdaterServer)
		if err != nil {
			fmt.Println("Server not found")
			time.Sleep(5 * time.Second)
			continue
		}
		cl.conn = conn
		err = cl.connect()
		if err != nil {
			fmt.Println(err, "sleep min.")
			cl.conn.Close()
			time.Sleep(time.Minute)
			continue
		}
		if exitCode := clientRun(); exitCode > 0 {
			fmt.Println("exitCode. sleep min.")
			cl.conn.Close()
			time.Sleep(time.Minute)
			continue
		}
		fmt.Println("No exitCode. sleep min.")
		cl.conn.Close()
		time.Sleep(time.Minute)
	}
}
