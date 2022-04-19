package web

import (
	"encoding/json"
	"fmt"
	"github.com/deepch/vdk/av"
	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
	. "videoPlayer/config"
	"videoPlayer/middleware"
	"videoPlayer/util"
)

type TestModel struct {
	Path      string `json:"path"`
	Policeman string `json:"policeman"`
	Prisoner  string `json:"prisoner"`
}

type RecordVideoModel struct {
	Filename string `json:"filename"`
	TaskId   string `json:"taskId"`
}

type JCodec struct {
	Type string
}

type RtspUrlDTO struct {
	RtspUrl      string `json:"rtspUrl" binding:"required"`
	DisableAudio bool   `json:"disableAudio"`
}

type ReceiverDTO struct {
	Data  string `json:"data"`
	Suuid string `json:"suuid"`
}

type ResponseDTO struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func (r *ResponseDTO) Success(msg string) *ResponseDTO {
	r.Code = 200
	r.Message = msg
	return r
}

func (r *ResponseDTO) SuccessWithData(msg string, data interface{}) *ResponseDTO {
	r.Code = 200
	r.Message = msg
	r.Data = data
	return r
}

// 路由
func ServeHTTP() {
	router := gin.Default()
	router.Use(middleware.Cors())

	router.GET("/ping", pong)

	// 流处理
	stream := router.Group("/stream")
	{
		stream.GET("/player/:uuid", HTTPAPIServerStreamPlayer)
		stream.POST("/receiver/:uuid", HTTPAPIServerStreamWebRTC)
		stream.GET("/codec/:uuid", HTTPAPIServerStreamCodec)
		stream.POST("/register", HTTPAPIServerStreamRegister)
	}

	// 静态文件代理
	router.StaticFS("/static", http.Dir("web/static"))

	// 判断 http 或 https

	if Config.Server.Ssl {
		// 启动https
		router.RunTLS(Config.Server.HTTPPort, Config.Server.SslPem, Config.Server.SslKey)
	} else {
		// 启动http
		err := router.Run(Config.Server.HTTPPort)
		if err != nil {
			log.Fatalln("启动http失败 ", err)
		}
	}
}

//HTTPAPIServerStreamPlayer stream player
func HTTPAPIServerStreamPlayer(c *gin.Context) {
	_, all := Config.List()
	sort.Strings(all)
	c.HTML(http.StatusOK, "player.tmpl", gin.H{
		"port":     Config.Server.HTTPPort,
		"suuid":    c.Param("uuid"),
		"suuidMap": all,
		"version":  time.Now().String(),
	})
}

//HTTPAPIServerStreamCodec stream codec
func HTTPAPIServerStreamCodec(c *gin.Context) {
	if Config.Ext(c.Param("uuid")) {
		Config.RunIFNotRun(c.Param("uuid"))
		codecs := Config.CoGe(c.Param("uuid"))
		if codecs == nil {
			return
		}
		var tmpCodec []JCodec
		for _, codec := range codecs {
			if codec.Type() != av.H264 && codec.Type() != av.PCM_ALAW && codec.Type() != av.PCM_MULAW && codec.Type() != av.OPUS {
				log.Println("Codec Not Supported WebRTC ignore this track", codec.Type())
				continue
			}
			if codec.Type().IsVideo() {
				tmpCodec = append(tmpCodec, JCodec{Type: "video"})
			} else {
				tmpCodec = append(tmpCodec, JCodec{Type: "audio"})
			}
		}
		b, err := json.Marshal(tmpCodec)
		if err == nil {
			_, err = c.Writer.Write(b)
			if err != nil {
				log.Println("Write Codec Info error", err)
				return
			}
		}
	}
}

//HTTPAPIServerStreamWebRTC stream video over WebRTC
func HTTPAPIServerStreamWebRTC(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")
	fmt.Println(contentType)

	var ssuid = ""
	var data = ""

	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		ssuid = c.PostForm("suuid")
		data = c.PostForm("data")
		if !Config.Ext(ssuid) {
			log.Println("Stream Not Found")
			return
		}
	} else if strings.Contains(contentType, "application/json") {
		var receiverDTO ReceiverDTO
		if err := c.ShouldBindJSON(&receiverDTO); err != nil {
			log.Println(err)
			c.JSON(200, "传递参数异常")
			return
		}
		ssuid = receiverDTO.Suuid
		data = receiverDTO.Data
	}

	Config.RunIFNotRun(ssuid)
	codecs := Config.CoGe(ssuid)
	if codecs == nil {
		log.Println("Stream Codec Not Found")
		return
	}
	var AudioOnly bool
	if len(codecs) == 1 && codecs[0].Type().IsAudio() {
		AudioOnly = true
	}
	muxerWebRTC := webrtc.NewMuxer(webrtc.Options{ICEServers: Config.GetICEServers(), PortMin: Config.GetWebRTCPortMin(), PortMax: Config.GetWebRTCPortMax()})
	answer, err := muxerWebRTC.WriteHeader(codecs, data)
	if err != nil {
		log.Println("WriteHeader", err)
		return
	}
	_, err = c.Writer.Write([]byte(answer))
	if err != nil {
		log.Println("Write", err)
		return
	}
	go func() {
		cid, ch := Config.ClAd(ssuid)
		defer Config.ClDe(ssuid, cid)
		defer muxerWebRTC.Close()
		var videoStart bool
		noVideo := time.NewTimer(10 * time.Second)
		for {
			select {
			case <-noVideo.C:
				log.Println("noVideo")
				return
			case pck := <-ch:
				if pck.IsKeyFrame || AudioOnly {
					noVideo.Reset(10 * time.Second)
					videoStart = true
				}
				if !videoStart && !AudioOnly {
					continue
				}
				err = muxerWebRTC.WritePacket(pck)
				if err != nil {
					log.Println("WritePacket", err)
					return
				}
			}
		}
	}()
}

//HTTPAPIServerStreamRegister register
func HTTPAPIServerStreamRegister(c *gin.Context) {
	var rtspUrlDTO RtspUrlDTO
	if err := c.ShouldBindJSON(&rtspUrlDTO); err != nil {
		log.Println(err)
		c.JSON(200, "传递rtspUrl异常")
		return
	}

	var responseDTO ResponseDTO
	log.Println("注册rtspUrl:", rtspUrlDTO.RtspUrl)

	// 检测是否注册过,注册过返回id
	cuuid, ok := RtspMap[rtspUrlDTO.RtspUrl]
	if ok {
		log.Println("该流已经注册过 cuuid:", cuuid)
		c.JSON(200, responseDTO.SuccessWithData("注册成功，等待播放", cuuid))
		return
	}

	// 为url生成唯一的id
	cuuid = PseudoUUID()
	log.Println("生成rtspUrl唯一id:", cuuid)

	streamST := StreamST{
		URL:          rtspUrlDTO.RtspUrl,
		OnDemand:     true,
		Cl:           make(map[string]Viewer),
		DisableAudio: rtspUrlDTO.DisableAudio,
	}

	// 添加到配置中
	Config.Streams[cuuid] = streamST
	RtspMap[rtspUrlDTO.RtspUrl] = cuuid
	log.Println("配置流:", Config.Streams[cuuid])

	c.JSON(200, responseDTO.SuccessWithData("注册成功，等待播放", cuuid))
	return

}

func pong(c *gin.Context) {
	c.JSON(http.StatusOK, util.Success("pong", ""))
}
