package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	app := &cli.App{
		Name:  "wechat-mp",
		Usage: "Get statistic info of wechat mp",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:     "token",
				Aliases:  []string{"t"},
				Required: true,
				Usage:    "Token used in URL",
			},
			&cli.StringFlag{
				Name:  "cookie-file",
				Usage: "Token file of wechat mp",
			},
			&cli.StringFlag{
				Name:  "o",
				Value: ".",
				Usage: "Output path of statistic data",
			},
			&cli.StringFlag{
				Name:  "dingtalk-token",
				Usage: "DingTalk token to send msg to robot",
			},
		},
		Action: func(cCtx *cli.Context) error {
			token := cCtx.Int("token")
			cookieFilePath := cCtx.String("cookie-file")
			outputPath := cCtx.String("o")
			dingTalkToken := cCtx.String("dingtalk-token")

			content, err := os.ReadFile(cookieFilePath)
			if err != nil {
				return err
			}
			cookie := strings.Split(string(content), "\n")[0]
			growDetails(token, cookie, outputPath, dingTalkToken)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

var lastStat = map[int64]postStat{}
var postMap = map[int64]postStat{}

type postStat struct {
	Time       int    `json:"time"`
	Title      string `json:"title"`
	ContentUrl string `json:"content_url"`
	Read       int    `json:"read"`
	Look       int    `json:"look"`
	Like       int    `json:"like"`
}

func growDetails(token int, cookie, outputPath, dingTalkToken string) {
	filename := filepath.Join(outputPath, fmt.Sprintf("%d", token))
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println(err)
	} else {
		err := json.Unmarshal(content, &lastStat)
		if err != nil {
			fmt.Println(err)
		}
	}

	getPageData(cookie, token, 0)

	var msg []string
	for key, val := range postMap {
		changed := true
		if _, exist := lastStat[key]; exist {
			if lastStat[key] == val {
				changed = false
			}
		} else {
			lastStat[key] = postStat{
				Read: 0,
				Look: 0,
				Like: 0,
			}
		}
		if changed {
			msg = append(msg, fmt.Sprintf("1. [%s](%s) %d/%d/%d => %d/%d/%d\r\n", val.Title, val.ContentUrl,
				lastStat[key].Read, lastStat[key].Look, lastStat[key].Like,
				val.Read, val.Look, val.Like))
		}
	}
	sendToDingTalk(msg, dingTalkToken)

	data, err := json.Marshal(postMap)
	if err != nil {
		fmt.Println(err)
	}
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0666)
	defer file.Close()
	_, err = file.Write(data)
	if err != nil {
		fmt.Println(err)
	}
}

func getPageData(cookie string, token, from int) {
	url := fmt.Sprintf("https://mp.weixin.qq.com/cgi-bin/appmsgpublish?sub=list&begin=%d&count=10&token=%d&lang=zh_CN", from, token)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("cookie", cookie)

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
	}
	total := parsePageData(string(body))
	if from+10 <= total {
		getPageData(cookie, token, from+10)
	}
}

type pageResponse struct {
	PublishList []struct {
		PublishInfo string `json:"publish_info"`
	} `json:"publish_list"`
	TotalCount int `json:"total_count"`
}

//type pageResponse struct {
//	FeaturedCount int `json:"featured_count"`
//	MasssendCount int `json:"masssend_count"`
//	PublishCount  int `json:"publish_count"`
//	PublishList   []struct {
//		PublishInfo string `json:"publish_info"`
//		PublishType int    `json:"publish_type"`
//	} `json:"publish_list"`
//	TotalCount int `json:"total_count"`
//}

type publishInfo struct {
	SentInfo struct {
		Time int `json:"time"`
	} `json:"sent_info"`
	AppmsgInfo []struct {
		Appmsgid   int64  `json:"appmsgid"`
		ContentUrl string `json:"content_url"`
		Title      string `json:"title"`
		ReadNum    int    `json:"read_num"`
		LikeNum    int    `json:"like_num"`
		Cover      string `json:"cover"`
		OldLikeNum int    `json:"old_like_num"`
		Digest     string `json:"digest"`
	} `json:"appmsg_info"`
	MsgId int `json:"msg_id"`
}

//type publishInfo struct {
//	Type     int `json:"type"`
//	Msgid    int `json:"msgid"`
//	SentInfo struct {
//		Time        int  `json:"time"`
//		FuncFlag    int  `json:"func_flag"`
//		IsSendAll   bool `json:"is_send_all"`
//		IsPublished int  `json:"is_published"`
//	} `json:"sent_info"`
//	SentStatus struct {
//		Total       int `json:"total"`
//		Succ        int `json:"succ"`
//		Fail        int `json:"fail"`
//		Progress    int `json:"progress"`
//		Userprotect int `json:"userprotect"`
//	} `json:"sent_status"`
//	SentResult struct {
//		MsgStatus       int           `json:"msg_status"`
//		RefuseReason    string        `json:"refuse_reason"`
//		RejectIndexList []interface{} `json:"reject_index_list"`
//		UpdateTime      int           `json:"update_time"`
//	} `json:"sent_result"`
//	AppmsgInfo []struct {
//		ShareType       int           `json:"share_type"`
//		Appmsgid        int64         `json:"appmsgid"`
//		ContentUrl      string        `json:"content_url"`
//		Title           string        `json:"title"`
//		IsDeleted       bool          `json:"is_deleted"`
//		CopyrightStatus int           `json:"copyright_status"`
//		CopyrightType   int           `json:"copyright_type"`
//		ReadNum         int           `json:"read_num"`
//		LikeNum         int           `json:"like_num"`
//		VoteIoteId      []interface{} `json:"vote_iote_id"`
//		Cover           string        `json:"cover"`
//		SmartProduct    int           `json:"smart_product"`
//		ModifyStatus    int           `json:"modify_status"`
//		AppmsgLikeType  int           `json:"appmsg_like_type"`
//		CanDeleteStatus int           `json:"can_delete_status"`
//		OldLikeNum      int           `json:"old_like_num"`
//		Itemidx         int           `json:"itemidx"`
//		IsPaySubscribe  int           `json:"is_pay_subscribe"`
//		IsFromTransfer  int           `json:"is_from_transfer"`
//		PublicTagInfo   struct {
//			PublicTagList   []interface{} `json:"public_tag_list"`
//			ModifyTimes     int           `json:"modify_times"`
//			InitTagListSize int           `json:"init_tag_list_size"`
//		} `json:"public_tag_info"`
//		AppmsgAlbumInfo struct {
//			AppmsgAlbumInfos []interface{} `json:"appmsg_album_infos"`
//		} `json:"appmsg_album_info"`
//		Digest           string `json:"digest"`
//		OpenFansmsg      int    `json:"open_fansmsg"`
//		IsCoolingArticle int    `json:"is_cooling_article"`
//	} `json:"appmsg_info"`
//	MsgId int `json:"msg_id"`
//}

func parsePageData(pageData string) int {
	listStr := strings.Split(strings.Split(pageData, "publish_page = ")[1], "};")[0] + "}"

	var pageResponse pageResponse
	err := json.Unmarshal([]byte(listStr), &pageResponse)
	if err != nil {
		fmt.Println(err)
		return -1
	}

	var publishInfo publishInfo
	for _, pageInfo := range pageResponse.PublishList {
		err := json.Unmarshal([]byte(strings.ReplaceAll(pageInfo.PublishInfo, "&quot;", "\"")), &publishInfo)
		if err != nil {
			fmt.Println(err)
			return -1
		}
		for _, appmsgInfo := range publishInfo.AppmsgInfo {
			postMap[appmsgInfo.Appmsgid] = postStat{
				Time:       publishInfo.SentInfo.Time,
				Title:      appmsgInfo.Title,
				ContentUrl: strings.ReplaceAll(appmsgInfo.ContentUrl, "&amp;", "&"),
				Read:       appmsgInfo.ReadNum,
				Look:       appmsgInfo.LikeNum,
				Like:       appmsgInfo.OldLikeNum,
			}
		}
	}

	return pageResponse.TotalCount
}

func sendToDingTalk(msg []string, dingTalkToken string) {
	if len(dingTalkToken) == 0 || len(msg) == 0 {
		return
	}
	url := "https://oapi.dingtalk.com/robot/send?access_token=" + dingTalkToken
	method := "POST"

	payload := strings.NewReader(`{
    "markdown": {
        "title": "公众号阅读量统计",
        "text": "` + strings.Join(msg, "") + `"
    },
    "msgtype": "markdown"
}`)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
}
