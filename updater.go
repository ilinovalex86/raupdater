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
	"sync"
	"time"
)

const configFile = "conf.txt"
const key = "2112751343910000"
const logFileName = "log.txt"

var clientExist = false
var conf config
var clientApp = "client"
var newClientApp = "newClient"
var command = "./" + clientApp

var l = logData{fileName: logFileName}

type logData struct {
	m        sync.Mutex
	fileName string
	eol      string
}

func toLog(data string, flag bool) {
	data = "updater " + time.Now().Format("02.01.2006 15:04:05") + " " + data
	l.m.Lock()
	file, err := os.OpenFile(l.fileName, os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal("open log file")
	}
	_, err = file.WriteString(data + l.eol)
	if err != nil {
		file.Close()
		log.Fatal("write data to log")
	}
	file.Close()
	l.m.Unlock()
	if flag {
		log.Fatal(data)
	}
}

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

func init() {
	const funcNameLog = "init(): "
	if !ex.ExistFile(l.fileName) {
		file, err := os.OpenFile(l.fileName, os.O_CREATE, 0666)
		if err != nil {
			log.Fatal("create log file")
		}
		file.Close()
	}
	if ex.ExistFile(configFile) {
		data, err := ex.ReadFileFull(configFile)
		if err != nil {
			toLog(funcNameLog+"read conf file", true)
		}
		err = json.Unmarshal(data, &conf)
		if err != nil {
			toLog(funcNameLog+"unmarshal conf file", true)
		}
	} else {
		conf := config{
			UpdaterServer: "127.0.0.1:50000",
			TcpServer:     "127.0.0.1:50001",
			StreamServer:  "127.0.0.1:50002",
			VersionClient: "0.0.0",
			ClientId:      "----------------",
		}
		data, err := json.MarshalIndent(&conf, "", "  ")
		if err != nil {
			toLog(funcNameLog+"marshal conf file", true)
		}
		err = ioutil.WriteFile(configFile, data, 0644)
		if err != nil {
			toLog(funcNameLog+"write conf file", true)
		}
		toLog("Файл конфигурации не найден. Создан новый файл конфигурации.", true)
	}
	if runtime.GOOS == "windows" {
		clientApp += ".exe"
		newClientApp += ".exe"
		path, err := os.Executable()
		if err != nil {
			toLog(funcNameLog+"os.Executable()", true)
		}
		i := strings.LastIndex(path, "\\")
		path = strings.Replace(path, path[i+1:], "", 1)
		clientApp = path + clientApp
		command = clientApp
		newClientApp = path + newClientApp
	}
	path, err := os.Stat(clientApp)
	if err != nil || path.IsDir() {
		fmt.Println("no client")
		conf.VersionClient = "-----"
	} else {
		clientExist = true
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
	const funcNameLog = "cl.downloadNewClient(): "
	cn.SendSync(cl.conn)
	err := cn.GetFile(q.Query, q.DataLen, cl.conn)
	if err != nil {
		return errors.New(funcNameLog + "downloadNewClient")
	}
	return nil
}

//Подключается к серверу и получаетот него указание
func (cl *clientData) connect() error {
	const funcNameLog = "cl.connect(): "
	if !cl.validOnServer(cl.conn) {
		toLog(funcNameLog+"Valid on Server", true)
	}
	err := cn.SendString(conf.ClientId, cl.conn)
	if err != nil {
		return errors.New(funcNameLog + "SendString(conf.ClientId, cl.conn)")
	}
	cn.ReadSync(cl.conn)
	jsonData, err := json.Marshal(cl)
	if err != nil {
		return errors.New(funcNameLog + "json.Marshal(cl)")
	}
	err = cn.SendBytesWithDelim(jsonData, cl.conn)
	if err != nil {
		return errors.New(funcNameLog + "SendBytesWithDelim(jsonData, cl.conn)")
	}
	q, err := cn.ReadQuery(cl.conn)
	if err != nil {
		return errors.New(funcNameLog + "ReadQuery(cl.conn)")
	}
	toLog(fmt.Sprintf("--> %#v", q), false)
	fmt.Printf("Query: %#v\n", q)
	switch q.Method {
	case "already exist":
		toLog(funcNameLog+"already exist", true)
	case "downloadNewClient":
		err = cl.downloadNewClient(q)
		if err != nil {
			return errors.New(funcNameLog + fmt.Sprint(err))
		}
		path, err := os.Stat(newClientApp)
		if err == nil && !path.IsDir() {
			toLog("update client", false)
			fmt.Println("update client")
			if clientExist {
				err = os.Remove(clientApp)
				if err != nil {
					return errors.New(funcNameLog + "os.Remove(clientApp)")
				}
			}
			time.Sleep(time.Second)
			err := os.Rename(newClientApp, clientApp)
			if err != nil {
				return errors.New(funcNameLog + "os.Rename(newClientApp, clientApp)")
			}
			time.Sleep(time.Second)
			if runtime.GOOS != "windows" {
				err = os.Chmod(clientApp, 0777)
				if err != nil {
					return errors.New(funcNameLog + "os.Chmod(clientApp, 0777)")
				}
				time.Sleep(time.Second)
			}
		} else {
			toLog(funcNameLog+"newClientApp: if err == nil && !path.IsDir()", true)
		}
		return nil
	case "lenClient":
		data, err := ex.ReadFileFull(clientApp)
		if err != nil {
			toLog(funcNameLog+"ReadFileFull(clientApp)", true)
		}
		if len(data) != q.DataLen {
			return errors.New(funcNameLog + "wrong client")
		}
		return nil
	case "getLog":
		data, err := ex.ReadFileFull(logFileName)
		if err != nil {
			err2 := cn.SendQuery(cn.Query{Query: "err read log file"}, cl.conn)
			if err2 != nil {
				return errors.New(funcNameLog + "cn.SendQuery(cn.Query{Query: \"err read log file\"}, cl.conn)")
			}
			return errors.New(funcNameLog + "ex.ReadFileFull(logFileName)")
		}
		err = cn.SendQuery(cn.Query{DataLen: len(data)}, cl.conn)
		if err != nil {
			return errors.New(funcNameLog + "cn.SendQuery(cn.Query{DataLen: len(fileBytes)}, cl.conn)")
		}
		cn.ReadSync(cl.conn)
		err = cn.SendBytes(data, cl.conn)
		if err != nil {
			return errors.New(funcNameLog + "cn.SendBytes(data, cl.conn)")
		}
		return errors.New("send logFile")
	}
	return nil
}

//Проходит валидацию при подключении к серверу
func (cl *clientData) validOnServer(conn net.Conn) bool {
	const funcNameLog = "cl.validOnServer(): "
	var code = make([]byte, 16)
	bc, err := aes.NewCipher([]byte(key))
	if err != nil {
		toLog(funcNameLog+"aes.NewCipher([]byte(key))", true)
	}
	err = cn.SendString("updater", conn)
	if err != nil {
		toLog(funcNameLog+"SendString(\"updater\", conn)", false)
		return false
	}
	data, err := cn.ReadBytesByLen(16, conn)
	if err != nil {
		toLog(funcNameLog+"ReadBytesByLen(16, conn)", false)
		return false
	}
	bc.Decrypt(code, data)
	s := string(code)
	res := s[len(s)/2:] + s[:len(s)/2]
	bc.Encrypt(code, []byte(res))
	err = cn.SendBytes(code, conn)
	if err != nil {
		toLog(funcNameLog+"SendBytes(code, conn)", false)
		return false
	}
	mes, err := cn.ReadString(conn)
	if err != nil {
		toLog(funcNameLog+"ReadString(conn)//res of valid", false)
		return false
	}
	if mes != "ok" {
		toLog(funcNameLog+"mes != \"ok\"", false)
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
	const funcNameLog = "main(): "
	cl := newClient()
	toLog("start updater", false)
	fmt.Println("Start updater")
	for {
		conn, err := net.Dial("tcp", conf.UpdaterServer)
		if err != nil {
			for i := 5; i >= 0; i-- {
				fmt.Printf("Server not found. Time to reconnect: %d\r", i)
				time.Sleep(1 * time.Second)
			}
			continue
		}
		toLog(fmt.Sprint("connect to conf.UpdaterServer: ", conf.UpdaterServer), false)
		cl.conn = conn
		err = cl.connect()
		if err != nil {
			toLog(funcNameLog+fmt.Sprint(err), false)
			fmt.Println(err, "sleep min.")
			cl.conn.Close()
			time.Sleep(time.Minute)
			continue
		}
		if exitCode := clientRun(); exitCode > 0 {
			toLog(funcNameLog+"exitCode. sleep min.", false)
			fmt.Println("exitCode. sleep min.")
			cl.conn.Close()
			time.Sleep(time.Minute)
			continue
		}
		toLog(funcNameLog+"No exitCode. sleep min.", false)
		fmt.Println("No exitCode. sleep min.")
		cl.conn.Close()
		time.Sleep(time.Minute)
	}
}
