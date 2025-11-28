package relay

import (
	"strconv"

	"relay-gateway/constant"
	"relay-gateway/relay/channel"
	"relay-gateway/relay/channel/ali"
	"relay-gateway/relay/channel/aws"
	"relay-gateway/relay/channel/baidu"
	"relay-gateway/relay/channel/baidu_v2"
	"relay-gateway/relay/channel/claude"
	"relay-gateway/relay/channel/cloudflare"
	"relay-gateway/relay/channel/coze"
	"relay-gateway/relay/channel/deepseek"
	"relay-gateway/relay/channel/dify"
	"relay-gateway/relay/channel/gemini"
	"relay-gateway/relay/channel/jimeng"
	"relay-gateway/relay/channel/minimax"
	"relay-gateway/relay/channel/mistral"
	"relay-gateway/relay/channel/mokaai"
	"relay-gateway/relay/channel/moonshot"
	"relay-gateway/relay/channel/ollama"
	"relay-gateway/relay/channel/openai"
	"relay-gateway/relay/channel/palm"
	"relay-gateway/relay/channel/perplexity"
	"relay-gateway/relay/channel/replicate"
	"relay-gateway/relay/channel/siliconflow"
	"relay-gateway/relay/channel/submodel"
	taskali "relay-gateway/relay/channel/task/ali"
	taskdoubao "relay-gateway/relay/channel/task/doubao"
	taskGemini "relay-gateway/relay/channel/task/gemini"
	"relay-gateway/relay/channel/task/hailuo"
	taskjimeng "relay-gateway/relay/channel/task/jimeng"
	"relay-gateway/relay/channel/task/kling"
	tasksora "relay-gateway/relay/channel/task/sora"
	taskVidu "relay-gateway/relay/channel/task/vidu"
	"relay-gateway/relay/channel/tencent"
	"relay-gateway/relay/channel/volcengine"
	"relay-gateway/relay/channel/xai"
	"relay-gateway/relay/channel/xunfei"
	"relay-gateway/relay/channel/zhipu"
	"relay-gateway/relay/channel/zhipu_4v"

	"github.com/gin-gonic/gin"
)

func GetAdaptor(apiType int) channel.Adaptor {
	switch apiType {
	case constant.APITypeAli:
		return &ali.Adaptor{}
	case constant.APITypeAnthropic:
		return &claude.Adaptor{}
	case constant.APITypeBaidu:
		return &baidu.Adaptor{}
	case constant.APITypeGemini:
		return &gemini.Adaptor{}
	case constant.APITypeOpenAI:
		return &openai.Adaptor{}
	case constant.APITypePaLM:
		return &palm.Adaptor{}
	case constant.APITypeTencent:
		return &tencent.Adaptor{}
	case constant.APITypeXunfei:
		return &xunfei.Adaptor{}
	case constant.APITypeZhipu:
		return &zhipu.Adaptor{}
	case constant.APITypeZhipuV4:
		return &zhipu_4v.Adaptor{}
	case constant.APITypeOllama:
		return &ollama.Adaptor{}
	case constant.APITypePerplexity:
		return &perplexity.Adaptor{}
	case constant.APITypeAws:
		return &aws.Adaptor{}
	case constant.APITypeDify:
		return &dify.Adaptor{}
	case constant.APITypeCloudflare:
		return &cloudflare.Adaptor{}
	case constant.APITypeSiliconFlow:
		return &siliconflow.Adaptor{}
	case constant.APITypeMistral:
		return &mistral.Adaptor{}
	case constant.APITypeDeepSeek:
		return &deepseek.Adaptor{}
	case constant.APITypeMokaAI:
		return &mokaai.Adaptor{}
	case constant.APITypeVolcEngine:
		return &volcengine.Adaptor{}
	case constant.APITypeBaiduV2:
		return &baidu_v2.Adaptor{}
	case constant.APITypeOpenRouter:
		return &openai.Adaptor{}
	case constant.APITypeXinference:
		return &openai.Adaptor{}
	case constant.APITypeXai:
		return &xai.Adaptor{}
	case constant.APITypeCoze:
		return &coze.Adaptor{}
	case constant.APITypeJimeng:
		return &jimeng.Adaptor{}
	case constant.APITypeMoonshot:
		return &moonshot.Adaptor{} // Moonshot uses Claude API
	case constant.APITypeSubmodel:
		return &submodel.Adaptor{}
	case constant.APITypeMiniMax:
		return &minimax.Adaptor{}
	case constant.APITypeReplicate:
		return &replicate.Adaptor{}
	}
	return nil
}

func GetTaskPlatform(c *gin.Context) constant.TaskPlatform {
	channelType := c.GetInt("channel_type")
	if channelType > 0 {
		return constant.TaskPlatform(strconv.Itoa(channelType))
	}
	return constant.TaskPlatform(c.GetString("platform"))
}

func GetTaskAdaptor(platform constant.TaskPlatform) channel.TaskAdaptor {
	switch platform {
	//case constant.APITypeAIProxyLibrary:
	//	return &aiproxy.Adaptor{}
	case constant.TaskPlatformSuno:
		return nil
	}
	if channelType, err := strconv.ParseInt(string(platform), 10, 64); err == nil {
		switch channelType {
		case constant.ChannelTypeAli:
			return &taskali.TaskAdaptor{}
		case constant.ChannelTypeKling:
			return &kling.TaskAdaptor{}
		case constant.ChannelTypeJimeng:
			return &taskjimeng.TaskAdaptor{}
		case constant.ChannelTypeVidu:
			return &taskVidu.TaskAdaptor{}
		case constant.ChannelTypeDoubaoVideo:
			return &taskdoubao.TaskAdaptor{}
		case constant.ChannelTypeSora, constant.ChannelTypeOpenAI:
			return &tasksora.TaskAdaptor{}
		case constant.ChannelTypeGemini:
			return &taskGemini.TaskAdaptor{}
		case constant.ChannelTypeMiniMax:
			return &hailuo.TaskAdaptor{}
		}
	}
	return nil
}
