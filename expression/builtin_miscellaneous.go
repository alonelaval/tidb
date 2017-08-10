// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
// // Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package expression

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/util/charset"
	"github.com/pingcap/tidb/util/types"
	"github.com/twinj/uuid"
)

var (
	_ functionClass = &sleepFunctionClass{}
	_ functionClass = &lockFunctionClass{}
	_ functionClass = &releaseLockFunctionClass{}
	_ functionClass = &anyValueFunctionClass{}
	_ functionClass = &defaultFunctionClass{}
	_ functionClass = &inetAtonFunctionClass{}
	_ functionClass = &inetNtoaFunctionClass{}
	_ functionClass = &inet6AtonFunctionClass{}
	_ functionClass = &inet6NtoaFunctionClass{}
	_ functionClass = &isFreeLockFunctionClass{}
	_ functionClass = &isIPv4FunctionClass{}
	_ functionClass = &isIPv4CompatFunctionClass{}
	_ functionClass = &isIPv4MappedFunctionClass{}
	_ functionClass = &isIPv6FunctionClass{}
	_ functionClass = &isUsedLockFunctionClass{}
	_ functionClass = &masterPosWaitFunctionClass{}
	_ functionClass = &nameConstFunctionClass{}
	_ functionClass = &releaseAllLocksFunctionClass{}
	_ functionClass = &uuidFunctionClass{}
	_ functionClass = &uuidShortFunctionClass{}
)

var (
	_ builtinFunc = &builtinSleepSig{}
	_ builtinFunc = &builtinLockSig{}
	_ builtinFunc = &builtinReleaseLockSig{}
	_ builtinFunc = &builtinAnyValueSig{}
	_ builtinFunc = &builtinDefaultSig{}
	_ builtinFunc = &builtinInetAtonSig{}
	_ builtinFunc = &builtinInetNtoaSig{}
	_ builtinFunc = &builtinInet6AtonSig{}
	_ builtinFunc = &builtinInet6NtoaSig{}
	_ builtinFunc = &builtinIsFreeLockSig{}
	_ builtinFunc = &builtinIsIPv4Sig{}
	_ builtinFunc = &builtinIsIPv4PrefixedSig{}
	_ builtinFunc = &builtinIsIPv6Sig{}
	_ builtinFunc = &builtinIsUsedLockSig{}
	_ builtinFunc = &builtinMasterPosWaitSig{}
	_ builtinFunc = &builtinNameConstSig{}
	_ builtinFunc = &builtinReleaseAllLocksSig{}
	_ builtinFunc = &builtinUUIDSig{}
	_ builtinFunc = &builtinUUIDShortSig{}
)

type sleepFunctionClass struct {
	baseFunctionClass
}

func (c *sleepFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf, err := newBaseBuiltinFuncWithTp(args, ctx, tpInt, tpReal)
	if err != nil {
		return nil, errors.Trace(err)
	}
	sig := &builtinSleepSig{baseIntBuiltinFunc{bf}}
	sig.deterministic = false
	return sig.setSelf(sig), nil
}

type builtinSleepSig struct {
	baseIntBuiltinFunc
}

// evalInt evals a builtinSleepSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_sleep
func (b *builtinSleepSig) evalInt(row []types.Datum) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalReal(row, b.ctx.GetSessionVars().StmtCtx)
	if err != nil {
		return 0, isNull, errors.Trace(err)
	}
	sessVars := b.ctx.GetSessionVars()
	if isNull {
		if sessVars.StrictSQLMode {
			return 0, true, errIncorrectArgs.GenByArgs("sleep")
		}
		return 0, true, nil
	}
	// processing argument is negative
	if val < 0 {
		if sessVars.StrictSQLMode {
			return 0, false, errIncorrectArgs.GenByArgs("sleep")
		}
		return 0, false, nil
	}

	// TODO: consider it's interrupted using KILL QUERY from other session, or
	// interrupted by time out.
	if val > math.MaxFloat64/float64(time.Second.Nanoseconds()) {
		return 0, false, errIncorrectArgs.GenByArgs("sleep")
	}
	dur := time.Duration(val * float64(time.Second.Nanoseconds()))
	time.Sleep(dur)
	return 0, false, nil
}

type lockFunctionClass struct {
	baseFunctionClass
}

func (c *lockFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinLockSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinLockSig struct {
	baseBuiltinFunc
}

// eval evals a builtinLockSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_get-lock
// The lock function will do nothing.
// Warning: get_lock() function is parsed but ignored.
func (b *builtinLockSig) eval(_ []types.Datum) (d types.Datum, err error) {
	d.SetInt64(1)
	return d, nil
}

type releaseLockFunctionClass struct {
	baseFunctionClass
}

func (c *releaseLockFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinReleaseLockSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinReleaseLockSig struct {
	baseBuiltinFunc
}

// eval evals a builtinReleaseLockSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_release-lock
// The release lock function will do nothing.
// Warning: release_lock() function is parsed but ignored.
func (b *builtinReleaseLockSig) eval(_ []types.Datum) (d types.Datum, err error) {
	d.SetInt64(1)
	return
}

type anyValueFunctionClass struct {
	baseFunctionClass
}

func (c *anyValueFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinAnyValueSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinAnyValueSig struct {
	baseBuiltinFunc
}

// eval evals a builtinAnyValueSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_any-value
func (b *builtinAnyValueSig) eval(row []types.Datum) (d types.Datum, err error) {
	args, err := b.evalArgs(row)
	if err != nil {
		return d, errors.Trace(err)
	}
	d = args[0]
	return d, nil
}

type defaultFunctionClass struct {
	baseFunctionClass
}

func (c *defaultFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinDefaultSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinDefaultSig struct {
	baseBuiltinFunc
}

// eval evals a builtinDefaultSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_default
func (b *builtinDefaultSig) eval(row []types.Datum) (d types.Datum, err error) {
	return d, errFunctionNotExists.GenByArgs("DEFAULT")
}

type inetAtonFunctionClass struct {
	baseFunctionClass
}

func (c *inetAtonFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinInetAtonSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinInetAtonSig struct {
	baseBuiltinFunc
}

// eval evals a builtinInetAtonSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_inet-aton
func (b *builtinInetAtonSig) eval(row []types.Datum) (d types.Datum, err error) {
	args, err := b.evalArgs(row)
	if err != nil {
		return d, errors.Trace(err)
	}
	arg := args[0]
	if arg.IsNull() {
		return d, nil
	}
	s, err := arg.ToString()
	if err != nil {
		return d, errors.Trace(err)
	}

	// ip address should not end with '.'.
	if len(s) == 0 || s[len(s)-1] == '.' {
		return d, nil
	}

	var (
		byteResult, result uint64
		dotCount           int
	)
	for _, c := range s {
		if c >= '0' && c <= '9' {
			digit := uint64(c - '0')
			byteResult = byteResult*10 + digit
			if byteResult > 255 {
				return d, nil
			}
		} else if c == '.' {
			dotCount++
			if dotCount > 3 {
				return d, nil
			}
			result = (result << 8) + byteResult
			byteResult = 0
		} else {
			return d, nil
		}
	}
	// 127 		-> 0.0.0.127
	// 127.255 	-> 127.0.0.255
	// 127.256	-> NULL
	// 127.2.1	-> 127.2.0.1
	switch dotCount {
	case 1:
		result <<= 8
		fallthrough
	case 2:
		result <<= 8
	}
	d.SetUint64((result << 8) + byteResult)
	return d, nil
}

type inetNtoaFunctionClass struct {
	baseFunctionClass
}

func (c *inetNtoaFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinInetNtoaSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinInetNtoaSig struct {
	baseBuiltinFunc
}

// eval evals a builtinInetNtoaSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_inet-ntoa
func (b *builtinInetNtoaSig) eval(row []types.Datum) (d types.Datum, err error) {
	args, err := b.evalArgs(row)
	if err != nil {
		return d, errors.Trace(err)
	}

	if args[0].IsNull() {
		return d, nil
	}

	ipArg, err := args[0].ToInt64(b.ctx.GetSessionVars().StmtCtx)
	if err != nil {
		return d, errors.Trace(err)
	}

	if ipArg < 0 || uint64(ipArg) > math.MaxUint32 {
		//not an IPv4 address.
		return d, nil
	}
	ip := make(net.IP, net.IPv4len)
	binary.BigEndian.PutUint32(ip, uint32(ipArg))
	ipv4 := ip.To4()
	if ipv4 == nil {
		//Not a vaild ipv4 address.
		return d, nil
	}

	d.SetString(ipv4.String())
	return d, nil
}

type inet6AtonFunctionClass struct {
	baseFunctionClass
}

func (c *inet6AtonFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinInet6AtonSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinInet6AtonSig struct {
	baseBuiltinFunc
}

// eval evals a builtinInet6AtonSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_inet6-aton
func (b *builtinInet6AtonSig) eval(row []types.Datum) (d types.Datum, err error) {
	args, err := b.evalArgs(row)
	if err != nil {
		return d, errors.Trace(err)
	}

	if args[0].IsNull() {
		return d, nil
	}

	ipAddress, err := args[0].ToString()
	if err != nil {
		return d, errors.Trace(err)
	}

	if len(ipAddress) == 0 {
		return d, nil
	}

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return d, nil
	}

	var isMappedIpv6 bool
	if ip.To4() != nil && strings.Contains(ipAddress, ":") {
		//mapped ipv6 address.
		isMappedIpv6 = true
	}

	var result []byte
	if isMappedIpv6 || ip.To4() == nil {
		result = make([]byte, net.IPv6len)
	} else {
		result = make([]byte, net.IPv4len)
	}

	if isMappedIpv6 {
		copy(result[12:], ip.To4())
		result[11] = 0xff
		result[10] = 0xff
	} else if ip.To4() == nil {
		copy(result, ip.To16())
	} else {
		copy(result, ip.To4())
	}

	d.SetBytes(result)
	return d, nil
}

type inet6NtoaFunctionClass struct {
	baseFunctionClass
}

func (c *inet6NtoaFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinInet6NtoaSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinInet6NtoaSig struct {
	baseBuiltinFunc
}

// eval evals a builtinInet6NtoaSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_inet6-ntoa
func (b *builtinInet6NtoaSig) eval(row []types.Datum) (d types.Datum, err error) {
	args, err := b.evalArgs(row)
	if err != nil {
		return d, errors.Trace(err)
	}

	if args[0].IsNull() {
		return d, nil
	}

	ipArg, err := args[0].ToBytes()
	if err != nil {
		return d, errors.Trace(err)
	}

	ip := net.IP(ipArg).String()
	if len(ipArg) == net.IPv6len && !strings.Contains(ip, ":") {
		ip = fmt.Sprintf("::ffff:%s", ip)
	}

	if net.ParseIP(ip) == nil {
		return d, nil
	}

	d.SetString(ip)

	return d, nil
}

type isFreeLockFunctionClass struct {
	baseFunctionClass
}

func (c *isFreeLockFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinIsFreeLockSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinIsFreeLockSig struct {
	baseBuiltinFunc
}

// eval evals a builtinIsFreeLockSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-free-lock
func (b *builtinIsFreeLockSig) eval(row []types.Datum) (d types.Datum, err error) {
	return d, errFunctionNotExists.GenByArgs("IS_FREE_LOCK")
}

type isIPv4FunctionClass struct {
	baseFunctionClass
}

func (c *isIPv4FunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf, err := newBaseBuiltinFuncWithTp(args, ctx, tpInt, tpString)
	if err != nil {
		return nil, errors.Trace(err)
	}
	bf.tp.Flen = 1
	bf.tp.Decimal = 0
	bf.tp.Charset = charset.CollationBin
	bf.tp.Flag |= mysql.BinaryFlag
	sig := &builtinIsIPv4Sig{baseIntBuiltinFunc{bf}}
	return sig.setSelf(sig), nil
}

type builtinIsIPv4Sig struct {
	baseIntBuiltinFunc
}

// evalInt evals a builtinIsIPv4Sig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-ipv4
func (b *builtinIsIPv4Sig) evalInt(row []types.Datum) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(row, b.ctx.GetSessionVars().StmtCtx)
	if err != nil || isNull {
		return int64(0), false, errors.Trace(err)
	}
	// isIPv4(str)
	// args[0] string
	if isIPv4(val) {
		return int64(1), false, nil
	}
	return int64(0), false, nil
}

// isIPv4 checks IPv4 address which satisfying the format A.B.C.D(0<=A/B/C/D<=255).
// Mapped IPv6 address like '::ffff:1.2.3.4' would return false.
func isIPv4(ip string) bool {
	// acc: keep the decimal value of each segment under check, which should between 0 and 255 for valid IPv4 address.
	// pd: sentinel for '.'
	dots, acc, pd := 0, 0, true
	for _, c := range ip {
		switch {
		case '0' <= c && c <= '9':
			acc = acc*10 + int(c-'0')
			pd = false
		case c == '.':
			dots++
			if dots > 3 || acc > 255 || pd {
				return false
			}
			acc, pd = 0, true
		default:
			return false
		}
	}
	if dots != 3 || acc > 255 || pd {
		return false
	}
	return true
}

type isIPv4CompatFunctionClass struct {
	baseFunctionClass
}

func (c *isIPv4CompatFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf, err := newBaseBuiltinFuncWithTp(args, ctx, tpInt, tpString)
	if err != nil {
		return nil, errors.Trace(err)
	}
	sig := &builtinIsIPv4PrefixedSig{baseIntBuiltinFunc{bf}, false}
	return sig.setSelf(sig), nil
}

type isIPv4MappedFunctionClass struct {
	baseFunctionClass
}

func (c *isIPv4MappedFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf, err := newBaseBuiltinFuncWithTp(args, ctx, tpInt, tpString)
	if err != nil {
		return nil, errors.Trace(err)
	}
	sig := &builtinIsIPv4PrefixedSig{baseIntBuiltinFunc{bf}, true}
	return sig.setSelf(sig), nil
}

type builtinIsIPv4PrefixedSig struct {
	baseIntBuiltinFunc
	// isIPv4MappedPrefix true for `Is_IPv4_Mapped`, false for `Is_IPv4_Compat`
	isIPv4MappedPrefix bool
}

// evalInt evals Is_IPv4_Mapped or Is_IPv4_Compat
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-ipv4-compat
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-ipv4-mapped
func (b *builtinIsIPv4PrefixedSig) evalInt(row []types.Datum) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(row, b.ctx.GetSessionVars().StmtCtx)
	if err != nil || isNull {
		return int64(0), false, errors.Trace(err)
	}
	var (
		prefixMapped = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}
		prefixCompat = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	)

	ipAddress := []byte(val)
	if len(ipAddress) != net.IPv6len {
		//Not an IPv6 address, return false
		return int64(0), false, nil
	}

	if b.isIPv4MappedPrefix {
		if !bytes.HasPrefix(ipAddress, prefixMapped) {
			return int64(0), false, nil
		}
	} else {
		if !bytes.HasPrefix(ipAddress, prefixCompat) {
			return int64(0), false, nil
		}
	}
	return int64(1), false, nil
}

type isIPv6FunctionClass struct {
	baseFunctionClass
}

func (c *isIPv6FunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf, err := newBaseBuiltinFuncWithTp(args, ctx, tpInt, tpString)
	if err != nil {
		return nil, errors.Trace(err)
	}
	sig := &builtinIsIPv6Sig{baseIntBuiltinFunc{bf}}
	return sig.setSelf(sig), nil
}

type builtinIsIPv6Sig struct {
	baseIntBuiltinFunc
}

// evalInt evals a builtinIsIPv6Sig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-ipv6
func (b *builtinIsIPv6Sig) evalInt(row []types.Datum) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(row, b.ctx.GetSessionVars().StmtCtx)
	if err != nil || isNull {
		return int64(0), false, errors.Trace(err)
	}
	ip := net.ParseIP(val)
	if ip != nil && !isIPv4(val) {
		return int64(1), false, nil
	}
	return int64(0), false, nil
}

type isUsedLockFunctionClass struct {
	baseFunctionClass
}

func (c *isUsedLockFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinIsUsedLockSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinIsUsedLockSig struct {
	baseBuiltinFunc
}

// eval evals a builtinIsUsedLockSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-used-lock
func (b *builtinIsUsedLockSig) eval(row []types.Datum) (d types.Datum, err error) {
	return d, errFunctionNotExists.GenByArgs("IS_USED_LOCK")
}

type masterPosWaitFunctionClass struct {
	baseFunctionClass
}

func (c *masterPosWaitFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinMasterPosWaitSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinMasterPosWaitSig struct {
	baseBuiltinFunc
}

// eval evals a builtinMasterPosWaitSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_master-pos-wait
func (b *builtinMasterPosWaitSig) eval(row []types.Datum) (d types.Datum, err error) {
	return d, errFunctionNotExists.GenByArgs("MASTER_POS_WAIT")
}

type nameConstFunctionClass struct {
	baseFunctionClass
}

func (c *nameConstFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinNameConstSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinNameConstSig struct {
	baseBuiltinFunc
}

// eval evals a builtinNameConstSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_name-const
func (b *builtinNameConstSig) eval(row []types.Datum) (d types.Datum, err error) {
	return d, errFunctionNotExists.GenByArgs("NAME_CONST")
}

type releaseAllLocksFunctionClass struct {
	baseFunctionClass
}

func (c *releaseAllLocksFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinReleaseAllLocksSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinReleaseAllLocksSig struct {
	baseBuiltinFunc
}

// eval evals a builtinReleaseAllLocksSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_release-all-locks
func (b *builtinReleaseAllLocksSig) eval(row []types.Datum) (d types.Datum, err error) {
	return d, errFunctionNotExists.GenByArgs("RELEASE_ALL_LOCKS")
}

type uuidFunctionClass struct {
	baseFunctionClass
}

func (c *uuidFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	bf, err := newBaseBuiltinFuncWithTp(args, ctx, tpString)
	if err != nil {
		return nil, errors.Trace(err)
	}
	bf.tp.Flen = 36
	bf.deterministic = false
	sig := &builtinUUIDSig{baseStringBuiltinFunc{bf}}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinUUIDSig struct {
	baseStringBuiltinFunc
}

// evalString evals a builtinUUIDSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_uuid
func (b *builtinUUIDSig) evalString(_ []types.Datum) (d string, isNull bool, err error) {
	return uuid.NewV1().String(), false, nil
}

type uuidShortFunctionClass struct {
	baseFunctionClass
}

func (c *uuidShortFunctionClass) getFunction(args []Expression, ctx context.Context) (builtinFunc, error) {
	sig := &builtinUUIDShortSig{newBaseBuiltinFunc(args, ctx)}
	return sig.setSelf(sig), errors.Trace(c.verifyArgs(args))
}

type builtinUUIDShortSig struct {
	baseBuiltinFunc
}

// eval evals a builtinUUIDShortSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_uuid-short
func (b *builtinUUIDShortSig) eval(row []types.Datum) (d types.Datum, err error) {
	return d, errFunctionNotExists.GenByArgs("UUID_SHORT")
}
