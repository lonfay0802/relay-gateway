package model

import (
	"errors"
	"fmt"
	"time"

	"relay-gateway/common"

	"gorm.io/gorm"
)

// WalletTransaction 钱包交易记录表
type WalletTransaction struct {
	ID               string     `json:"id" gorm:"column:id;type:varchar(32);primaryKey;not null"`
	UserID           string     `json:"user_id" gorm:"column:user_id;type:varchar(32);not null;index"`
	Type             string     `json:"type" gorm:"column:type;type:varchar(20);not null;index"` // recharge(储值)、deduction(扣款)、refund(退款)
	AmountCents      int        `json:"amount_cents" gorm:"column:amount_cents;type:int4;not null"` // 正数为入账，负数为出账
	BalanceBeforeCents int      `json:"balance_before_cents" gorm:"column:balance_before_cents;type:int4;not null"` // 变化前余额
	BalanceAfterCents int       `json:"balance_after_cents" gorm:"column:balance_after_cents;type:int4;not null"` // 变化后余额
	RelatedID        *string    `json:"related_id" gorm:"column:related_id;type:varchar(32);index"` // 关联的订单ID，充值记录Id或api调用记录id
	Description      string     `json:"description" gorm:"column:description;type:text"`
	CreatedAt        time.Time  `json:"created_at" gorm:"column:created_at;type:timestamptz(6);default:now();index"`
	TransactionNumber string    `json:"transaction_number" gorm:"column:transaction_number;type:varchar(50);not null;uniqueIndex"`
	UpdatedAt        *time.Time `json:"updated_at" gorm:"column:updated_at;type:timestamptz(6);default:now()"`
	Status           string     `json:"status" gorm:"column:status;type:varchar(20);default:'completed';index"` // pending、completed、failed
	RelatedType      string     `json:"related_type" gorm:"column:related_type;type:varchar(20);index"` // recharge_order: 储值订单、api_usage: API调用、refund_request: 退款
}

// TableName 指定表名
func (WalletTransaction) TableName() string {
	return "t_wallet_transactions"
}

// 交易类型常量
const (
	TransactionTypeRecharge = "recharge" // 储值
	TransactionTypeDeduction = "deduction" // 扣款
	TransactionTypeRefund   = "refund"   // 退款
)

// 交易状态常量
const (
	TransactionStatusPending   = "pending"   // 待处理
	TransactionStatusCompleted = "completed" // 已完成
	TransactionStatusFailed    = "failed"    // 失败
)

// 关联类型常量
const (
	RelatedTypeRechargeOrder = "recharge_order" // 储值订单
	RelatedTypeAPIUsage      = "api_usage"      // API调用
	RelatedTypeRefundRequest = "refund_request" // 退款请求
	RelatedTypeSystem        = "system"         // 系统操作（如补扣费、退款等）
)

// generateTransactionNumber 生成交易编号
// 格式: TXN + 时间戳(14位) + 随机字符串(8位)
func generateTransactionNumber() string {
	return fmt.Sprintf("TXN%s%s", common.GetTimeString(), common.GetRandomString(8))
}

// CreateWalletTransaction 创建钱包交易记录
// 注意：此函数不会自动更新钱包余额，需要在调用前先更新钱包余额
func CreateWalletTransaction(tx *gorm.DB, userId string, transactionType string, amountCents int, balanceBeforeCents int, description string, relatedID *string, relatedType string) (*WalletTransaction, error) {
	if userId == "" {
		return nil, errors.New("user id 为空")
	}
	if transactionType == "" {
		return nil, errors.New("交易类型不能为空")
	}
	if amountCents == 0 {
		return nil, errors.New("交易金额不能为0")
	}

	// 验证交易类型
	validTypes := map[string]bool{
		TransactionTypeRecharge: true,
		TransactionTypeDeduction: true,
		TransactionTypeRefund:   true,
	}
	if !validTypes[transactionType] {
		return nil, fmt.Errorf("无效的交易类型: %s", transactionType)
	}

	// 计算变化后余额
	balanceAfterCents := balanceBeforeCents + amountCents

	// 验证余额不能为负数（扣款时）
	if transactionType == TransactionTypeDeduction && balanceAfterCents < 0 {
		return nil, errors.New("余额不足，无法完成扣款")
	}

	transaction := &WalletTransaction{
		ID:                common.GetUUID(),
		UserID:            userId,
		Type:              transactionType,
		AmountCents:       amountCents,
		BalanceBeforeCents: balanceBeforeCents,
		BalanceAfterCents: balanceAfterCents,
		RelatedID:         relatedID,
		Description:       description,
		TransactionNumber: generateTransactionNumber(),
		Status:            TransactionStatusCompleted,
		RelatedType:       relatedType,
		CreatedAt:         time.Now(),
		UpdatedAt:         common.GetTimestampTz(),
	}

	db := tx
	if db == nil {
		db = DB
	}

	err := db.Create(transaction).Error
	if err != nil {
		return nil, fmt.Errorf("创建交易记录失败: %w", err)
	}

	return transaction, nil
}

// CreateWalletTransactionWithBalanceUpdate 创建钱包交易记录并更新钱包余额（事务操作）
// 这是一个便捷函数，会在一个事务中完成余额更新和交易记录创建
func CreateWalletTransactionWithBalanceUpdate(userId string, transactionType string, amountCents int, description string, relatedID *string, relatedType string) (*WalletTransaction, error) {
	if userId == "" {
		return nil, errors.New("user id 为空")
	}

	var transaction *WalletTransaction

	// 开始事务
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 获取当前钱包余额
		var wallet UserWallets
		err := tx.Where("user_id = ?", userId).First(&wallet).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("钱包记录不存在")
			}
			return err
		}

		balanceBeforeCents := wallet.BalanceCents

		// 根据交易类型更新余额
		switch transactionType {
		case TransactionTypeRecharge:
			// 充值：增加余额和累计充值
			err = tx.Model(&UserWallets{}).
				Where("user_id = ?", userId).
				Updates(map[string]interface{}{
					"balance_cents":        gorm.Expr("balance_cents + ?", amountCents),
					"total_recharged_cents": gorm.Expr("total_recharged_cents + ?", amountCents),
				}).Error
		case TransactionTypeDeduction:
			// 扣款：减少余额，增加累计消费
			if wallet.BalanceCents < amountCents {
				return errors.New("余额不足，无法完成扣款")
			}
			err = tx.Model(&UserWallets{}).
				Where("user_id = ?", userId).
				Updates(map[string]interface{}{
					"balance_cents":     gorm.Expr("balance_cents - ?", amountCents),
					"total_spent_cents": gorm.Expr("total_spent_cents + ?", amountCents),
				}).Error
		case TransactionTypeRefund:
			// 退款：增加余额
			err = tx.Model(&UserWallets{}).
				Where("user_id = ?", userId).
				Update("balance_cents", gorm.Expr("balance_cents + ?", amountCents)).Error
		default:
			return fmt.Errorf("无效的交易类型: %s", transactionType)
		}

		if err != nil {
			return fmt.Errorf("更新钱包余额失败: %w", err)
		}

		// 创建交易记录
		transaction, err = CreateWalletTransaction(tx, userId, transactionType, amountCents, balanceBeforeCents, description, relatedID, relatedType)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return transaction, nil
}

// GetWalletTransactionsByUserId 根据用户ID获取交易记录
func GetWalletTransactionsByUserId(userId string, limit int, offset int) ([]*WalletTransaction, int64, error) {
	if userId == "" {
		return nil, 0, errors.New("user id 为空")
	}

	var transactions []*WalletTransaction
	var total int64

	// 获取总数
	err := DB.Model(&WalletTransaction{}).
		Where("user_id = ?", userId).
		Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 获取记录
	err = DB.Where("user_id = ?", userId).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error
	if err != nil {
		return nil, 0, err
	}

	return transactions, total, nil
}

// GetWalletTransactionByID 根据交易ID获取交易记录
func GetWalletTransactionByID(transactionID string) (*WalletTransaction, error) {
	if transactionID == "" {
		return nil, errors.New("交易ID为空")
	}

	var transaction WalletTransaction
	err := DB.Where("id = ?", transactionID).First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("交易记录不存在")
		}
		return nil, err
	}

	return &transaction, nil
}

// GetWalletTransactionByNumber 根据交易编号获取交易记录
func GetWalletTransactionByNumber(transactionNumber string) (*WalletTransaction, error) {
	if transactionNumber == "" {
		return nil, errors.New("交易编号为空")
	}

	var transaction WalletTransaction
	err := DB.Where("transaction_number = ?", transactionNumber).First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("交易记录不存在")
		}
		return nil, err
	}

	return &transaction, nil
}

// UpdateTransactionStatus 更新交易状态
func UpdateTransactionStatus(transactionID string, status string) error {
	if transactionID == "" {
		return errors.New("交易ID为空")
	}

	validStatuses := map[string]bool{
		TransactionStatusPending:   true,
		TransactionStatusCompleted: true,
		TransactionStatusFailed:    true,
	}
	if !validStatuses[status] {
		return fmt.Errorf("无效的交易状态: %s", status)
	}

	return DB.Model(&WalletTransaction{}).
		Where("id = ?", transactionID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": common.GetTimestampTz(),
		}).Error
}

