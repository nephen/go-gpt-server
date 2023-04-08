package token

import (
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

func CreateToken(userid string, duration time.Duration, secretkey string) (string, error) {
	// 创建一个JWT Claims
	claims := jwt.StandardClaims{
		ExpiresAt: time.Now().Add(duration).Unix(),
		Id:        userid,
	}

	// 创建一个JWT Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 使用密钥进行签名
	tokenString, err := token.SignedString([]byte(secretkey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ValidateToken(tokenString string, secretkey string) (string, error) {
	// 解析JWT Token
	token, err := jwt.ParseWithClaims(tokenString, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法和密钥
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretkey), nil
	})

	if err != nil {
		return "", err
	}

	// 提取JWT Claims中的用户ID
	if claims, ok := token.Claims.(*jwt.StandardClaims); ok && token.Valid {
		return claims.Id, nil
	} else {
		return "", fmt.Errorf("invalid token")
	}
}
