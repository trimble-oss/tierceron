package kafkautil

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"time"
	// mysql and mssql go libraries
)

// OpenConnection opens connection to a database using various sql urls.
func OpenConnection(broker string, username string, password string) (*sql.DB, error) {
	return nil, nil
}

// Float64frombytes -- converts byte array to float.
func Float64frombytes(bytes []byte) float64 {
	bits := binary.LittleEndian.Uint64(bytes)
	float := math.Float64frombits(bits)
	return float
}

// RatFromFloat -- converts byte array to float.
func RatFromFloat(x float64, scale int) *big.Rat {
	scaleFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	num := big.NewInt(int64(x * float64(scaleFactor.Int64())))
	denom := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	r := new(big.Rat).SetFrac(num, denom)

	return r
}

// RandomString - generate a random string of specfied length
func RandomString(min int, max int) string {
	rand.Seed(time.Now().UnixNano())
	randomInt := rand.Intn(max-min+1) + min + 1
	lengthAsString := strconv.Itoa(len(strconv.Itoa(max)))
	return fmt.Sprintf("%0"+lengthAsString+"d", randomInt)
}
