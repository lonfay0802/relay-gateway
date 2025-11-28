package service

import (
	"relay-gateway/common"
	"relay-gateway/model"
	"relay-gateway/setting/ratio_setting"
)

const (
	BillingModeToken    = "token"
	BillingModeCell     = "cell"
	BillingModeDuration = "duration"
)

// LoadModelRatiosFromDB 从数据库加载模型倍率配置并更新到内存
func LoadModelRatiosFromDB() error {
	// 获取所有有效的定价配置
	pricings, err := model.GetAllActivePricing()
	if err != nil {
		common.SysError("failed to load model pricing from database: " + err.Error())
		return err
	}

	// 构建各种倍率映射
	modelRatioMap := make(map[string]float64)
	completionRatioMap := make(map[string]float64)
	cacheRatioMap := make(map[string]float64)
	imageRatioMap := make(map[string]float64)
	audioRatioMap := make(map[string]float64)
	audioCompletionRatioMap := make(map[string]float64)
	modelPriceMap := make(map[string]float64)

	inputTokenPriceMap := make(map[string]float64)
	outputTokenPriceMap := make(map[string]float64)
	modelBillingMap := make(map[string]string)

	// 遍历定价配置，构建映射
	// 注意：如果同一个 model_id 有多条记录，后面的会覆盖前面的
	// 这里已经按 effective_from DESC 排序，所以最新的会覆盖旧的
	for _, pricing := range pricings {
		modelID := pricing.ModelID

		// 更新模型倍率
		if pricing.ModelRatio > 0 {
			modelRatioMap[modelID] = float64(pricing.ModelRatio)
		}

		// 更新完成倍率
		if pricing.CompletionRatio > 0 {
			completionRatioMap[modelID] = float64(pricing.CompletionRatio)
		}

		// 更新缓存倍率
		if pricing.CacheRatio > 0 {
			cacheRatioMap[modelID] = float64(pricing.CacheRatio)
		}

		// 更新图片倍率
		if pricing.ImageRatio > 0 {
			imageRatioMap[modelID] = float64(pricing.ImageRatio)
		}

		// 更新音频倍率
		if pricing.AudioRatio > 0 {
			audioRatioMap[modelID] = float64(pricing.AudioRatio)
		}

		// 更新音频完成倍率
		if pricing.AudioCompletionRatio > 0 {
			audioCompletionRatioMap[modelID] = float64(pricing.AudioCompletionRatio)
		}

		modelBillingMap[modelID] = pricing.BillingMode

		// 根据计费模式计算价格
		// 如果 billing_mode 是 "call"，使用 price_per_call_cents
		// 如果 billing_mode 是 "token"，可以根据 input_price_per_1k_tokens 和 output_price_per_1k_tokens 计算
		// 这里简化处理，如果有 price_per_call_cents，则使用它
		if pricing.BillingMode == BillingModeCell && pricing.PricePerCallCents > 0 {
			// 将毫分转换为价格（假设 1 单位 = 1 毫分 / 10000，即 1 元 = 10000 毫分）
			modelPriceMap[modelID] = float64(pricing.PricePerCallCents)
		}

		if pricing.BillingMode == BillingModeToken {
			inputTokenPriceMap[modelID] = float64(pricing.InputPricePer1kTokens)
			outputTokenPriceMap[modelID] = float64(pricing.OutputPricePer1kTokens)
		}
	}

	// 更新内存中的映射
	if len(modelRatioMap) > 0 {
		err = ratio_setting.UpdateModelRatioFromMap(modelRatioMap)
		if err != nil {
			common.SysError("failed to update model ratio map: " + err.Error())
		}
	}

	if len(completionRatioMap) > 0 {
		err = ratio_setting.UpdateCompletionRatioFromMap(completionRatioMap)
		if err != nil {
			common.SysError("failed to update completion ratio map: " + err.Error())
		}
	}

	if len(cacheRatioMap) > 0 {
		err = ratio_setting.UpdateCacheRatioFromMap(cacheRatioMap)
		if err != nil {
			common.SysError("failed to update cache ratio map: " + err.Error())
		}
	}

	if len(imageRatioMap) > 0 {
		err = ratio_setting.UpdateImageRatioFromMap(imageRatioMap)
		if err != nil {
			common.SysError("failed to update image ratio map: " + err.Error())
		}
	}

	if len(audioRatioMap) > 0 {
		err = ratio_setting.UpdateAudioRatioFromMap(audioRatioMap)
		if err != nil {
			common.SysError("failed to update audio ratio map: " + err.Error())
		}
	}

	if len(audioCompletionRatioMap) > 0 {
		err = ratio_setting.UpdateAudioCompletionRatioFromMap(audioCompletionRatioMap)
		if err != nil {
			common.SysError("failed to update audio completion ratio map: " + err.Error())
		}
	}

	if len(modelPriceMap) > 0 {
		err = ratio_setting.UpdateModelPriceFromMap(modelPriceMap)
		if err != nil {
			common.SysError("failed to update model price map: " + err.Error())
		}
	}

	if len(modelBillingMap) > 0 {
		err = ratio_setting.UpdateModelBillingModeFromMap(modelBillingMap)
		if err != nil {
			common.SysError("failed to update model billing mode: " + err.Error())
		}
	}

	if len(inputTokenPriceMap) > 0 {
		err = ratio_setting.UpdateInputTokenPriceFromMap(inputTokenPriceMap)
		if err != nil {
			common.SysError("failed to update input token price map: " + err.Error())
		}
	}

	if len(outputTokenPriceMap) > 0 {
		err = ratio_setting.UpdateOutputTokenPriceFromMap(outputTokenPriceMap)
		if err != nil {
			common.SysError("failed to update output token price map: " + err.Error())
		}
	}

	// 加载默认分组倍率到内存缓存
	err = model.LoadDefaultGroupRatiosToCache()
	if err != nil {
		common.SysError("failed to load default group ratios to cache: " + err.Error())
	} else {
		common.SysLog("loaded default group ratios to cache successfully")
	}

	common.SysLog("loaded model ratios from database successfully")
	return nil
}

// ReloadModelRatiosFromDB 重新从数据库加载模型倍率配置
func ReloadModelRatiosFromDB() error {
	return LoadModelRatiosFromDB()
}
