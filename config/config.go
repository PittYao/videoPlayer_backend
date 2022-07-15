package config

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/deepch/vdk/av"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"sync"
	"time"
)

//ConfigST struct
type ConfigST struct {
	mutex    sync.RWMutex
	Server   ServerST            `json:"server"`
	Database Database            `json:"database"`
	Streams  map[string]StreamST `json:"streams"`
	WebRTC   WebRTC              `json:"webRTC"`
}

//ServerST struct
type ServerST struct {
	HTTPPort      string   `json:"http_port"`
	HTTPSPort     string   `json:"https_port"`
	SslKey        string   `json:"ssl_key"`
	SslPem        string   `json:"ssl_pem"`
	ICEServers    []string `json:"ice_servers"`
	WebRTCPortMin uint16   `json:"webrtc_port_min"`
	WebRTCPortMax uint16   `json:"webrtc_port_max"`
}

//Database struct
type Database struct {
	Driver string `json:"driver"`
	Url    string `json:"url"`
}

//StreamST struct
type StreamST struct {
	URL          string `json:"url"`
	Status       bool   `json:"status"`
	OnDemand     bool   `json:"on_demand"`
	DisableAudio bool   `json:"disableAudio"`
	RunLock      bool   `json:"-"`
	Codecs       []av.CodecData
	Cl           map[string]Viewer
}

var RtspMap = map[string]string{}

//WebRTC struct
type WebRTC struct {
	OpenTurn    OpenTurn    `json:"openTurn"`
	PrivateTurn PrivateTurn `json:"privateTurn"`
	EnableTurn  bool        `json:"enableTurn"` // true = 外网 false = 内网
}

type OpenTurn struct {
	URL        string `json:"url"`
	Credential string `json:"credential"`
	Username   string `json:"username"`
}

type PrivateTurn struct {
	URL        string `json:"url"`
	Credential string `json:"credential"`
	Username   string `json:"username"`
}

type Viewer struct {
	c chan av.Packet
}

func (element *ConfigST) RunIFNotRun(uuid string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	if tmp, ok := element.Streams[uuid]; ok {
		if tmp.OnDemand && !tmp.RunLock {
			tmp.RunLock = true
			element.Streams[uuid] = tmp
			go RTSPWorkerLoop(uuid, tmp.URL, tmp.OnDemand, tmp.DisableAudio)
		}
	}
}

func (element *ConfigST) RunUnlock(uuid string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	if tmp, ok := element.Streams[uuid]; ok {
		if tmp.OnDemand && tmp.RunLock {
			tmp.RunLock = false
			element.Streams[uuid] = tmp
		}
	}
}

func (element *ConfigST) HasViewer(uuid string) bool {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	if tmp, ok := element.Streams[uuid]; ok && len(tmp.Cl) > 0 {
		return true
	}
	return false
}

func (element *ConfigST) GetICEServers() []string {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.ICEServers
}

func (element *ConfigST) GetWebRTCPortMin() uint16 {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.WebRTCPortMin
}

func (element *ConfigST) GetWebRTCPortMax() uint16 {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.WebRTCPortMax
}

func (element *ConfigST) Cast(uuid string, pck av.Packet) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	for _, v := range element.Streams[uuid].Cl {
		if len(v.c) < cap(v.c) {
			v.c <- pck
		}
	}
}

func (element *ConfigST) Ext(suuid string) bool {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	_, ok := element.Streams[suuid]
	return ok
}

func (element *ConfigST) CoAd(suuid string, codecs []av.CodecData) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	t := element.Streams[suuid]
	t.Codecs = codecs
	element.Streams[suuid] = t
}

func (element *ConfigST) CoGe(suuid string) []av.CodecData {
	for i := 0; i < 100; i++ {
		element.mutex.RLock()
		tmp, ok := element.Streams[suuid]
		element.mutex.RUnlock()
		if !ok {
			return nil
		}
		if tmp.Codecs != nil {
			return tmp.Codecs
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (element *ConfigST) ClAd(suuid string) (string, chan av.Packet) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	cuuid := PseudoUUID()
	ch := make(chan av.Packet, 100)
	element.Streams[suuid].Cl[cuuid] = Viewer{c: ch}
	return cuuid, ch
}

func (element *ConfigST) List() (string, []string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	var res []string
	var fist string
	for k := range element.Streams {
		if fist == "" {
			fist = k
		}
		res = append(res, k)
	}
	return fist, res
}

func (element *ConfigST) ClDe(suuid, cuuid string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	delete(element.Streams[suuid].Cl, cuuid)
}

//初始化配置文件 和 数据库连接
var Config, Db = loadConfig()

func loadConfig() (*ConfigST, *gorm.DB) {
	var tmp ConfigST
	data, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Error().Msgf("启动服务失败,读取配置文件失败")
		panic("启动服务失败,读取配置文件失败")
	}
	err = json.Unmarshal(data, &tmp)
	if err != nil {
		log.Error().Msgf("启动服务失败,配置文件内容异常")
		panic("启动服务失败,配置文件内容异常")
	}
	for i, v := range tmp.Streams {
		v.Cl = make(map[string]Viewer)
		tmp.Streams[i] = v
	}

	// connect db
	if tmp.Database.Url == "" {
		log.Warn().Msgf("配置文件中没有配置数据库url")
		return &tmp, nil
	} else {
		db, err := gorm.Open(tmp.Database.Driver, tmp.Database.Url)
		// 取消复数表模式
		db.SingularTable(true)
		// 打印sql
		db.LogMode(true)

		if err != nil {
			log.Error().Msgf("启动服务失败,连接数据库失败")
			//panic("database open fail: " + err.Error())
		}
		return &tmp, db
	}

}

func PseudoUUID() (uuid string) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return
}
