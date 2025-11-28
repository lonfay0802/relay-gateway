package model

import (
	"errors"
	"relay-gateway/common"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type UserEnhance struct {
	Id          string     `json:"id"`
	Username    string     `json:"username" gorm:"type:varchar(100);index" validate:"max=32"`
	Password    string     `json:"password_hash" gorm:"type:varchar(255);" validate:"min=8,max=255"`
	Email       string     `json:"email" gorm:"type:varchar(255);index" validate:"max=50"`
	DisplayName string     `json:"display_name" gorm:"type:varchar(255);" `
	AvatarUrl   string     `json:"avatar_url" gorm:"type:varchar(1000);"`
	Provider    string     `json:"provider" gorm:"type:varchar(20)"`
	ProviderId  string     `json:"provider_id" gorm:"type:varchar(255)"`
	Status      int        `json:"status" gorm:"default:1" `
	Phone       string     `json:"phone" gorm:"type:varchar(11)"`
	Deleted     int        `json:"deleted" gorm:"default:0"`
	CreatedAt   *time.Time `json:"created_at" gorm:"type:timestamptz(6);default:now()"` // 创建时间
	UpdatedAt   *time.Time `json:"updated_at" gorm:"type:timestamptz(6);default:now()"` // 更新时间
	LastLoginAt *time.Time `json:"last_login_at" gorm:"type:timestamptz(6)"`            // 最后使用时间
	Group       string     `json:"group_name" gorm:"column:group_name;type:varchar(255);default:'default'"`
	Quota       int        `json:"quota" gorm:"type:int;default:0"`
}

func (user *UserEnhance) ToBaseUser() *UserBase {
	cache := &UserBase{
		Id:       user.Id,
		Group:    user.Group,
		Quota:    user.Quota,
		Status:   user.Status,
		Username: user.Username,
		Email:    user.Email,
	}
	return cache
}

func GetUserByIdEnhance(id string, selectAll bool) (*UserEnhance, error) {
	if id == "" {
		return nil, errors.New("id 为空！")
	}
	user := UserEnhance{Id: id}
	var err error = nil
	// 明确指定要查询的字段，排除 Quota（因为 Quota 不在 t_users 表中）
	selectFields := "id,username,email,display_name,avatar_url,provider,provider_id,status,phone,deleted,created_at,updated_at,last_login_at,group_name"
	if selectAll {
		selectFields = "id,username,password,email,display_name,avatar_url,provider,provider_id,status,phone,deleted,created_at,updated_at,last_login_at,group_name"
	}
	err = DB.Table("t_users").Select(selectFields).First(&user, "id = ?", id).Error
	if err != nil {
		return &user, err
	}

	// 从钱包表获取余额并赋值给 Quota
	wallet, err := GetUserWalletByUserId(id)
	if err != nil {
		// 如果钱包不存在，Quota 保持为 0（默认值）
		if err.Error() != "钱包记录不存在" {
			// 如果是其他错误，记录日志但不影响返回用户信息
			common.SysLog("failed to get user wallet: " + err.Error())
		}
	} else {
		user.Quota = wallet.BalanceCents
	}

	return &user, nil
}

func DecreaseUserQuota(id string, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	gopool.Go(func() {
		err := cacheDecrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to decrease user quota: " + err.Error())
		}
	})
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, -quota)
		return nil
	}
	return decreaseUserQuota(id, quota)
}

func decreaseUserQuota(id string, quota int) (err error) {
	// 扣减钱包余额
	err = DecreaseUserWalletBalance(id, quota)
	if err != nil {
		return err
	}
	return err
}

func GetUserQuota(id string, fromDB bool) (quota int, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserQuotaCache(id, quota); err != nil {
					common.SysLog("failed to update user quota cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		quota, err := getUserQuotaCache(id)
		if err == nil {
			return quota, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Table("t_user_wallets").Model(&UserWallets{}).Where("user_id = ?", id).Select("balance_cents").Find(&quota).Error
	if err != nil {
		return 0, err
	}

	return quota, nil
}

func updateUserUsedQuotaAndRequestCount(id string, quota int, count int) {
	err := DB.Table("t_users").Model(&UserEnhance{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", count),
		},
	).Error
	if err != nil {
		common.SysLog("failed to update user used quota and request count: " + err.Error())
		return
	}

	//// 更新缓存
	//if err := invalidateUserCache(id); err != nil {
	//	common.SysError("failed to invalidate user cache: " + err.Error())
	//}
}

func UpdateUserUsedQuotaAndRequestCount(id string, quota int) {
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUsedQuota, id, quota)
		addNewRecord(BatchUpdateTypeRequestCount, id, 1)
		return
	}
	updateUserUsedQuotaAndRequestCount(id, quota, 1)
}

func IncreaseUserQuota(id string, quota int, db bool) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	gopool.Go(func() {
		err := cacheIncrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to increase user quota: " + err.Error())
		}
	})
	if !db && common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, quota)
		return nil
	}
	return increaseUserQuota(id, quota)
}

func increaseUserQuota(id string, quota int) (err error) {
	// 增加钱包余额
	err = IncreaseUserWalletBalance(id, quota)
	if err != nil {
		return err
	}
	return err
}

func updateUserUsedQuota(id string, quota int) {
	err := DB.Table("t_users").Model(&UserEnhance{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota": gorm.Expr("used_quota + ?", quota),
		},
	).Error
	if err != nil {
		common.SysLog("failed to update user used quota: " + err.Error())
	}
}

func updateUserRequestCount(id string, count int) {
	err := DB.Table("t_users").Model(&UserEnhance{}).Where("id = ?", id).Update("request_count", gorm.Expr("request_count + ?", count)).Error
	if err != nil {
		common.SysLog("failed to update user request count: " + err.Error())
	}
}

func RootUserExists() bool {
	var user UserEnhance
	// 只查询 id 字段即可，因为只是检查是否存在
	err := DB.Table("t_users").Select("id").Where("role = ?", common.RoleRootUser).First(&user).Error
	if err != nil {
		return false
	}
	return true
}

func GetUserById(id string, selectAll bool) (*UserEnhance, error) {
	if id == "" {
		return nil, errors.New("id 为空！")
	}
	user := UserEnhance{Id: id}
	var err error = nil
	// 明确指定要查询的字段，排除 Quota（因为 Quota 不在 t_users 表中）
	selectFields := "id,username,email,display_name,avatar_url,provider,provider_id,status,phone,deleted,created_at,updated_at,last_login_at,group_name"
	if selectAll {
		selectFields = "id,username,password,email,display_name,avatar_url,provider,provider_id,status,phone,deleted,created_at,updated_at,last_login_at,group_name"
	}
	err = DB.Table("t_users").Select(selectFields).First(&user, "id = ?", id).Error
	return &user, err
}

func GetUsernameById(id string, fromDB bool) (username string, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserNameCache(id, username); err != nil {
					common.SysLog("failed to update user name cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		username, err := getUserNameCache(id)
		if err == nil {
			return username, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Table("t_users").Model(&UserEnhance{}).Where("id = ?", id).Select("username").Find(&username).Error
	if err != nil {
		return "", err
	}

	return username, nil
}
