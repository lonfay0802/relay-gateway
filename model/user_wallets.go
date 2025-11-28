package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// UserWallets 用户钱包表，用于存储用户账户余额
type UserWallets struct {
	Id                  string     `json:"id" gorm:"type:varchar(32);primaryKey;not null"`
	UserId              string     `json:"user_id" gorm:"type:varchar(32);not null;index;column:user_id"`
	BalanceCents        int        `json:"balance_cents" gorm:"type:int4;default:0;column:balance_cents"`                 // 账户余额，单位：美分
	TotalRechargedCents int        `json:"total_recharged_cents" gorm:"type:int4;default:0;column:total_recharged_cents"` // 累计充值
	TotalSpentCents     int        `json:"total_spent_cents" gorm:"type:int4;default:0;column:total_spent_cents"`         // 累计消费
	Status              string     `json:"status" gorm:"type:varchar(20);default:'active';column:status"`                 // 状态：active, inactive等
	FrozenCents         *int       `json:"frozen_cents" gorm:"type:int4;column:frozen_cents"`                             // 冻结的金额，可为空
	CreatedAt           *time.Time `json:"created_at" gorm:"type:timestamptz(6);default:now();column:created_at"`         // 创建时间
	UpdatedAt           *time.Time `json:"updated_at" gorm:"type:timestamptz(6);default:now();column:updated_at"`         // 更新时间
}

// TableName 指定表名
func (UserWallets) TableName() string {
	return "t_user_wallets"
}

// GetAvailableBalance 获取可用余额（余额 - 冻结金额）
func (w *UserWallets) GetAvailableBalance() int {
	frozen := 0
	if w.FrozenCents != nil {
		frozen = *w.FrozenCents
	}
	return w.BalanceCents - frozen
}

// GetUserWalletByUserId 根据用户ID获取钱包信息
func GetUserWalletByUserId(userId string) (*UserWallets, error) {
	if userId == "" {
		return nil, errors.New("user id 为空")
	}
	var wallet UserWallets
	err := DB.Where("user_id = ?", userId).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("钱包记录不存在")
		}
		return nil, err
	}
	return &wallet, nil
}

// GetUserWalletByUserIdOrCreate 根据用户ID获取钱包信息，如果不存在则创建
func GetUserWalletByUserIdOrCreate(userId string) (*UserWallets, error) {
	if userId == "" {
		return nil, errors.New("user id 为空")
	}
	var wallet UserWallets
	err := DB.Where("user_id = ?", userId).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 如果钱包不存在，创建一个新的钱包记录
			wallet = UserWallets{
				UserId:              userId,
				BalanceCents:        0,
				TotalRechargedCents: 0,
				TotalSpentCents:     0,
				Status:              "active",
				FrozenCents:         nil,
			}
			if err := DB.Create(&wallet).Error; err != nil {
				return nil, err
			}
			return &wallet, nil
		}
		return nil, err
	}
	return &wallet, nil
}

// CreateUserWallet 创建用户钱包
func CreateUserWallet(userId string, initialBalanceCents int) (*UserWallets, error) {
	if userId == "" {
		return nil, errors.New("user id 为空")
	}
	wallet := UserWallets{
		UserId:              userId,
		BalanceCents:        initialBalanceCents,
		TotalRechargedCents: initialBalanceCents,
		TotalSpentCents:     0,
		Status:              "active",
		FrozenCents:         nil,
	}
	if err := DB.Create(&wallet).Error; err != nil {
		return nil, err
	}
	return &wallet, nil
}

// UpdateUserWalletBalance 更新用户钱包余额
func UpdateUserWalletBalance(userId string, balanceCents int) error {
	if userId == "" {
		return errors.New("user id 为空")
	}
	return DB.Model(&UserWallets{}).Where("user_id = ?", userId).Update("balance_cents", balanceCents).Error
}

// IncreaseUserWalletBalance 增加用户钱包余额（同时更新累计充值）
func IncreaseUserWalletBalance(userId string, amountCents int) error {
	if userId == "" {
		return errors.New("user id 为空")
	}
	if amountCents < 0 {
		return errors.New("amount_cents 不能为负数")
	}
	return DB.Model(&UserWallets{}).
		Where("user_id = ?", userId).
		Updates(map[string]interface{}{
			"balance_cents": gorm.Expr("balance_cents + ?", amountCents),
		}).Error
}

// DecreaseUserWalletBalance 减少用户钱包余额（同时更新累计消费）
func DecreaseUserWalletBalance(userId string, amountCents int) error {
	if userId == "" {
		return errors.New("user id 为空")
	}
	if amountCents < 0 {
		return errors.New("amount_cents 不能为负数")
	}
	return DB.Model(&UserWallets{}).
		Where("user_id = ?", userId).
		Updates(map[string]interface{}{
			"balance_cents":     gorm.Expr("balance_cents - ?", amountCents),
			"total_spent_cents": gorm.Expr("total_spent_cents + ?", amountCents),
		}).Error
}

// FreezeUserWalletBalance 冻结用户钱包余额
func FreezeUserWalletBalance(userId string, amountCents int) error {
	if userId == "" {
		return errors.New("user id 为空")
	}
	if amountCents < 0 {
		return errors.New("amount_cents 不能为负数")
	}
	return DB.Model(&UserWallets{}).
		Where("user_id = ?", userId).
		Update("frozen_cents", gorm.Expr("COALESCE(frozen_cents, 0) + ?", amountCents)).Error
}

// UnfreezeUserWalletBalance 解冻用户钱包余额
func UnfreezeUserWalletBalance(userId string, amountCents int) error {
	if userId == "" {
		return errors.New("user id 为空")
	}
	if amountCents < 0 {
		return errors.New("amount_cents 不能为负数")
	}
	return DB.Model(&UserWallets{}).
		Where("user_id = ?", userId).
		Update("frozen_cents", gorm.Expr("GREATEST(COALESCE(frozen_cents, 0) - ?, 0)", amountCents)).Error
}

// UpdateUserWalletStatus 更新用户钱包状态
func UpdateUserWalletStatus(userId string, status string) error {
	if userId == "" {
		return errors.New("user id 为空")
	}
	return DB.Model(&UserWallets{}).Where("user_id = ?", userId).Update("status", status).Error
}

// GetUserWalletBalance 获取用户钱包余额（单位：美分）
func GetUserWalletBalance(userId string) (int, error) {
	wallet, err := GetUserWalletByUserId(userId)
	if err != nil {
		return 0, err
	}
	return wallet.BalanceCents, nil
}

// GetUserWalletAvailableBalance 获取用户钱包可用余额（余额 - 冻结金额，单位：美分）
func GetUserWalletAvailableBalance(userId string) (int, error) {
	wallet, err := GetUserWalletByUserId(userId)
	if err != nil {
		return 0, err
	}
	return wallet.GetAvailableBalance(), nil
}
