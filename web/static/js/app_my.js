let suuid = $('#suuid').val();

let pc = null;

let sendChannel = null;

let stream = null;

let pcObj = {
    pc: pc,
    stream: stream,
    sendChannel: sendChannel
}

function pcReload() {
    pcClear();

    let stream = new MediaStream();
    pcObj.stream = stream

    let config = {
        iceServers: [{
            urls: ["stun:stun.l.google.com:19302"]
        }]
    };

    pcObj.pc = new RTCPeerConnection(config);
    pcObj.pc.onnegotiationneeded = handleNegotiationNeededEvent;

    let log = msg => {
        document.getElementById('div').innerHTML += msg + '<br>'
    }

    pcObj.pc.ontrack = function (event) {
        pcObj.stream.addTrack(event.track);
        console.log(pcObj.stream)
        videoElem.srcObject = pcObj.stream;
        log(event.streams.length + ' track is delivered')
    }

    pcObj.pc.oniceconnectionstatechange = e => log(pcObj.pc.iceConnectionState)

    async function handleNegotiationNeededEvent() {
        let offer = await pcObj.pc.createOffer();
        await pcObj.pc.setLocalDescription(offer);
        getRemoteSdp();
    }

}

function pcClear() {
    videoElem.srcObject = null;
    pcObj.stream = null;
    pcObj.sendChannel = null;
    pcObj.pc = null
    console.log("清空pc");
}

function getCodecInfo() {
    $.get("../stream/codec/" + suuid, function (data) {
        try {
            console.log(data)
            data = JSON.parse(data);
        } catch (e) {
            console.log(e);
        } finally {
            $.each(data, function (index, value) {
                pcObj.pc.addTransceiver(value.Type, {
                    'direction': 'sendrecv'
                })
            })
            //send ping becouse PION not handle RTCSessionDescription.close()
            pcObj.sendChannel = pcObj.pc.createDataChannel('foo');
            pcObj.sendChannel.onclose = () => console.log('sendChannel has closed');
            pcObj.sendChannel.onopen = () => {
                console.log('sendChannel has opened');
                pcObj.sendChannel.send('ping');
                setInterval(() => {
                    pcObj.sendChannel.send('ping');
                }, 1000)
            }
            pcObj.sendChannel.onmessage = e => log(`Message from DataChannel '${pcObj.sendChannel.label}' payload '${e.data}'`);
        }
    });
}

function getRemoteSdp() {
    $.post("../stream/receiver/" + suuid, {
        suuid: suuid,
        data: btoa(pcObj.pc.localDescription.sdp)
    }, function (data) {
        try {
            pcObj.pc.setRemoteDescription(new RTCSessionDescription({
                type: 'answer',
                sdp: atob(data)
            }))
        } catch (e) {
            console.warn(e);
        }
    });
}

function playVideo() {
    // 重置
    pcReload();
    // 清空
    console.log("pc状态：", pcObj.pc.connectionState);
    if (pcObj.pc.connectionState === "connected") {
        // 连接状态断开
        pcObj.pc.close();
        console.log("pc关闭后建立状态：", pcObj.pc.connectionState);
    }

    let rtspUrl = $('#rtspUrlInput').val();
    console.log("rtspUrl:", rtspUrl)
    // 请求注册接口
    var send_data = {
        "rtspUrl": rtspUrl,
        "disableAudio": true,
    }
    var jsonData = JSON.stringify(send_data)
    $.ajax({
        type: "post",
        url:'../stream/register',
        dataType: 'json',
        data: jsonData,
        contentType:'application/json',
        success: function (jsonData) {
            try {
                console.log(jsonData)
                if (jsonData.code === 200) {
                    console.log(jsonData.message)
                    // 设置id
                    $('#suuid').val(jsonData.data)
                    suuid = $('#suuid').val();
                    console.log("suuid:", suuid)
                    // 请求编解码
                    getCodecInfo();
                }
            } catch (e) {
                console.warn("注册失败:", e);
            }
        }

    })
}
