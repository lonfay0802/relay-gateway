package dto

import (
	"encoding/json"
)

type TaskError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Data       any    `json:"data"`
	StatusCode int    `json:"-"`
	LocalError bool   `json:"-"`
	Error      error  `json:"-"`
}

type SunoDataResponse struct {
	TaskID     string          `json:"task_id" gorm:"type:varchar(50);index"`
	Action     string          `json:"action" gorm:"type:varchar(40);index"` // 任务类型, song, lyrics, description-mode
	Status     string          `json:"status" gorm:"type:varchar(20);index"` // 任务状态, submitted, queueing, processing, success, failed
	FailReason string          `json:"fail_reason"`
	SubmitTime int64           `json:"submit_time" gorm:"index"`
	StartTime  int64           `json:"start_time" gorm:"index"`
	FinishTime int64           `json:"finish_time" gorm:"index"`
	Data       json.RawMessage `json:"data" gorm:"type:json"`
}

type TaskData interface {
	SunoDataResponse | []SunoDataResponse | string | any
}

type TaskResponse[T TaskData] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type TaskDto struct {
	TaskID     string          `json:"task_id"` // 第三方id，不一定有/ song id\ Task id
	Action     string          `json:"action"`  // 任务类型, song, lyrics, description-mode
	Status     string          `json:"status"`  // 任务状态, submitted, queueing, processing, success, failed
	FailReason string          `json:"fail_reason"`
	SubmitTime int64           `json:"submit_time"`
	StartTime  int64           `json:"start_time"`
	FinishTime int64           `json:"finish_time"`
	Progress   string          `json:"progress"`
	Data       json.RawMessage `json:"data"`
}

const TaskSuccessCode = "success"

func (t *TaskResponse[T]) IsSuccess() bool {
	return t.Code == TaskSuccessCode
}
