package lua

/*
#cgo windows,!llua LDFLAGS: -lm -lws2_32
#cgo linux,!llua LDFLAGS: -lm
#cgo darwin,!llua LDFLAGS: -lm

//#cgo CFLAGS: -DUSING_PTHREAD=1
#cgo CFLAGS: -DCUSTOM_LUA_LOCK=0
#include "clua.h"
#include "c-lib.h"
#include <lua.h>
#include <lauxlib.h>
#include <lualib.h>
#include <stdio.h>
#include <stdlib.h>
static int xlua_gc (lua_State *L, int what, int data) {
    return lua_gc(L, what, data);
}
*/
import "C"

import (
    "fmt"
    "log"
    "reflect"
    "sync"
    "sync/atomic"
    "unsafe"
)

const (
    LUA_MULTRET = C.LUA_MULTRET
)

const (
    LUA_TNONE          = int(C.LUA_TNONE)
    LUA_TNIL           = int(C.LUA_TNIL)
    LUA_TNUMBER        = int(C.LUA_TNUMBER)
    LUA_TBOOLEAN       = int(C.LUA_TBOOLEAN)
    LUA_TSTRING        = int(C.LUA_TSTRING)
    LUA_TTABLE         = int(C.LUA_TTABLE)
    LUA_TFUNCTION      = int(C.LUA_TFUNCTION)
    LUA_TUSERDATA      = int(C.LUA_TUSERDATA)
    LUA_TTHREAD        = int(C.LUA_TTHREAD)
    LUA_TLIGHTUSERDATA = int(C.LUA_TLIGHTUSERDATA)
)

type (
    GoFunction func(L *State) int
    State      struct {
        s          *C.lua_State
        name       string
        registryId uint32
        registerM  sync.Mutex
        registry   map[uint32]interface{} // go object registry to uint32
        closeChan  chan struct{}
        closeOnce  sync.Once
        lock       sync.Mutex
        gcCount    int64
    }
    Error struct {
        code       int
        message    string
        stackTrace []StackEntry
    }
    StackEntry struct {
        Name        string
        Source      string
        ShortSource string
        CurrentLine int
    }
)

func (e *Error) Error() string {
    return e.message
}

var (
    goStates      map[interface{}]*State
    goStatesMutex sync.Mutex
    namedStates   map[string]*State
)

//export luago_lock
func luago_lock(L *C.lua_State) {}

//export luago_unlock
func luago_unlock(L *C.lua_State) {}
func init() {
    goStates = make(map[interface{}]*State, 16)
    namedStates = make(map[string]*State, 16)
}
func newState(L *C.lua_State) *State {
    st := &State{
        s:          L,
        registryId: 0,
        registry:   make(map[uint32]interface{}),
        closeChan:  make(chan struct{}),
    }
    registerGoState(st)
    C.c_initstate(st.s)
    st.PushGoStruct(st)
    st.SetGlobal("_LuaState")
    return st
}
func registerGoState(L *State) {
    goStatesMutex.Lock()
    defer goStatesMutex.Unlock()
    goStates[L.s] = L
}
func unregisterGoState(L *State) {
    goStatesMutex.Lock()
    defer goStatesMutex.Unlock()
    delete(goStates, L.s)
}
func getGoState(L *C.lua_State) *State {
    goStatesMutex.Lock()
    defer goStatesMutex.Unlock()
    st, _ := goStates[L]
    return st
}
func getNamedState(name string) *State {
    goStatesMutex.Lock()
    defer goStatesMutex.Unlock()
    st, _ := namedStates[name]
    return st
}
func setNamedState(name string, L *State) {
    goStatesMutex.Lock()
    defer goStatesMutex.Unlock()
    namedStates[name] = L
}

// Lua State
func NewState(Ls ...*C.lua_State) *State {
    var L *C.lua_State
    if len(Ls) == 1 {
        L = Ls[0]
    }
    if L == nil {
        L = (C.luaL_newstate())
        if L == nil {
            return nil
        }
    }
    return newState(L)
}
func (L *State) Close() {
    L.closeOnce.Do(func() {
        C.lua_close(L.s)
        unregisterGoState(L)
        close(L.closeChan)
    })
}
func (L *State) CloseChan() <-chan struct{} { // expose readonly chan
    return L.closeChan
}
func (L *State) DoFile(filename string) error {
    if r := L.LoadFile(filename); r != 0 {
        return &Error{
            code:    -1,
            message: fmt.Sprintf("run file error:%s", filename),
        }
    }
    return L.Call(0, LUA_MULTRET)
}
func (L *State) LoadFile(filename string) int {
    Cfilename := C.CString(filename)
    defer C.free(unsafe.Pointer(Cfilename))
    return int(C.luaL_loadfilex(L.s, Cfilename, nil))
}
func (L *State) Type(idx int) int {
    return int(C.lua_type(L.s, C.int(idx)))
}
func (L *State) Typename(typeId int) string {
    return C.GoString(C.lua_typename(L.s, C.int(typeId)))
}
func (L *State) TypenameX(idx int) string {
    return L.Typename(L.Type(idx))
}
func (L *State) Call(nargs int, nresults int) (err error) {
    defer func() {
        if err2 := recover(); err2 != nil {
            if _, ok := err2.(error); ok {
                err = err2.(error)
            }
            return
        }
    }()

    r := C.lua_pcallk(L.s, C.int(nargs), C.int(nresults), 0, 0, nil)
    if r != 0 {
        return &Error{
            code:       int(r),
            message:    L.ToString(-1),
            stackTrace: L.StackTrace(),
        }
    }
    return nil
}
func (L *State) CallX(cbRef int, autoUnref bool, nResults int, inArgs ...interface{}) (err error) {
    L.RawGetiX(cbRef)
    if !L.IsLuaFunction(-1) {
        return &Error{
            code:       -1,
            message:    "try invoke non function",
            stackTrace: L.StackTrace(),
        }
    }

    n := 0
    for _, arg := range inArgs {
        fval := reflect.ValueOf(arg)
        switch fval.Kind() {
        case reflect.Bool:
            n++
            L.PushBoolean(fval.Bool())
        case reflect.String:
            n++
            L.PushString(fval.String())
        case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
            n++
            L.PushInteger(fval.Int())
        case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
            n++
            L.PushInteger(int64(fval.Uint()))
        case reflect.Float32, reflect.Float64:
            n++
            L.PushNumber(fval.Float())
        case reflect.Slice:
            n++
            L.PushBytes(fval.Bytes())
        case reflect.Ptr, reflect.Interface:
            n++
            if arg == nil {
                L.PushNil()
            } else {
                L.PushGoStruct(arg)
            }
        }
    }
    err = L.Call(n, nResults)
    if autoUnref {
        L.UnrefX(cbRef)
    }
    return
}
func (L *State) CheckStack() (result []interface{}) {
    n := L.GetTop()
    for idx := 1; idx <= n; idx++ {
        luatype := L.Type(idx)

        switch luatype {
        case LUA_TNUMBER:
            result = append(result, L.ToNumber(idx))
        case LUA_TSTRING:
            result = append(result, L.ToString(idx))
        case LUA_TUSERDATA:
            result = append(result, L.ToGoStruct(idx))
        case LUA_TBOOLEAN:
            result = append(result, L.ToInteger(idx) == 1)
        case LUA_TNIL:
            result = append(result, nil)
        }
    }
    return
}
func (L *State) DoString(str string) error {
    if r := L.LoadString(str); r != 0 {
        return &Error{
            code:       r,
            message:    L.ToString(-1),
            stackTrace: L.StackTrace(),
        }
    }
    return L.Call(0, 0)
}
func (L *State) LoadString(str string) int {
    Cs := C.CString(str)
    defer C.free(unsafe.Pointer(Cs))
    return int(C.luaL_loadstring(L.s, Cs))
}
func (L *State) GetTop() int  { return int(C.lua_gettop(L.s)) }
func (L *State) SetTop(n int) { C.lua_settop(L.s, C.int(n)) }
func (L *State) StackTrace() []StackEntry {
    var r []StackEntry
    var d C.lua_Debug
    Sln := C.CString("Sln")
    defer C.free(unsafe.Pointer(Sln))

    for depth := 0; C.lua_getstack(L.s, C.int(depth), &d) > 0; depth++ {
        C.lua_getinfo(L.s, Sln, &d)
        ssb := make([]byte, C.LUA_IDSIZE)
        for i := 0; i < C.LUA_IDSIZE; i++ {
            ssb[i] = byte(d.short_src[i])
            if ssb[i] == 0 {
                ssb = ssb[:i]
                break
            }
        }
        ss := string(ssb)

        r = append(r, StackEntry{C.GoString(d.name), C.GoString(d.source), ss, int(d.currentline)})
    }

    return r
}
func (L *State) OpenLibs() {
    C.luaL_openlibs(L.s)
}
func (L *State) register(f interface{}) (id uint32) {
    L.registerM.Lock()
    defer L.registerM.Unlock()

    for {
        id = atomic.AddUint32(&L.registryId, 1)
        if _, ok := L.registry[id]; ok {
            continue
        }
        if id == 0 { // ????????????uint32
            id = atomic.AddUint32(&L.registryId, 1)
        }
        L.registry[id] = f
        break
    }
    return id
}
func (L *State) getRegister(id uint32) interface{} {
    L.registerM.Lock()
    defer L.registerM.Unlock()
    if p, ok := L.registry[id]; ok {
        return p
    }
    return nil
}
func (L *State) delRegister(id uint32) interface{} {
    L.registerM.Lock()
    defer L.registerM.Unlock()
    if p, ok := L.registry[id]; ok {
        delete(L.registry, id)
        return p
    }
    return nil
}
func (L *State) RegisterLib(name string, fn unsafe.Pointer) {
    Sln := C.CString(name)
    defer C.free(unsafe.Pointer(Sln))
    C.c_register_lib(L.s, fn, Sln)
}
func (L *State) OpenLibsExt() {
    L.RegisterLib("serialize", C.luaopen_serialize)
    L.RegisterLib("cmsgpack", C.luaopen_cmsgpack)
    L.RegisterLib("pb", C.luaopen_pb)
    L.RegisterLib("cjson", C.luaopen_cjson)
}
func (L *State) Ref(t int) int {
    return int(C.luaL_ref(L.s, C.int(t)))
}
func (L *State) Unref(t int, ref int) {
    C.luaL_unref(L.s, C.int(t), C.int(ref))
}
func (L *State) RefX() int {
    return L.Ref(C.LUA_REGISTRYINDEX)
}
func (L *State) UnrefX(ref int) {
    L.Unref(C.LUA_REGISTRYINDEX, ref)
}
func (L *State) RawGetiX(ref int) {
    L.RawGeti(C.LUA_REGISTRYINDEX, ref)
}
func (L *State) New() *State {
    return NewState(nil)
}
func (L *State) SetName(name string) {
    L.name = name
    setNamedState(name, L)
}
func (L *State) GetName() string {
    return L.name
}
func (L *State) SendMessage(name string, cmd int, data []byte) {
    go func() {
        t := getNamedState(name)
        if t == nil {
            return
        }
        t.GetGlobal("_LuaStateMessage")
        if t.IsLuaFunction(-1) {
            t.PushInteger(int64(cmd))
            t.PushBytes(data)
            if err := t.Call(2, 0); err != nil {
                log.Println(err)
            }
        } else {
            log.Println("not func")
        }
    }()
}
func (L *State) NewThread() *State {
    nL := C.lua_newthread(L.s)
    return NewState(nL)
}
func (L *State) WaitClose() {
    defer func() {
        recover()
    }()
    select {
    case <-L.closeChan:
    }
}
func (L *State) Lock() {
    L.lock.Lock()
}
func (L *State) Unlock() {
    L.lock.Unlock()
}

// Is
func (L *State) IsGoFunction(index int) bool { return C.c_is_gostruct(L.s, C.int(index)) != 0 }
func (L *State) IsGoStruct(index int) bool   { return C.c_is_gostruct(L.s, C.int(index)) != 0 }
func (L *State) IsBoolean(index int) bool    { return int(C.lua_type(L.s, C.int(index))) == LUA_TBOOLEAN }
func (L *State) IsLightUserdata(index int) bool {
    return int(C.lua_type(L.s, C.int(index))) == LUA_TLIGHTUSERDATA
}
func (L *State) IsNil(index int) bool       { return int(C.lua_type(L.s, C.int(index))) == LUA_TNIL }
func (L *State) IsNone(index int) bool      { return int(C.lua_type(L.s, C.int(index))) == LUA_TNONE }
func (L *State) IsNoneOrNil(index int) bool { return int(C.lua_type(L.s, C.int(index))) <= 0 }
func (L *State) IsNumber(index int) bool    { return C.lua_isnumber(L.s, C.int(index)) == 1 }
func (L *State) IsString(index int) bool    { return C.lua_isstring(L.s, C.int(index)) == 1 }
func (L *State) IsTable(index int) bool     { return int(C.lua_type(L.s, C.int(index))) == LUA_TTABLE }
func (L *State) IsThread(index int) bool    { return int(C.lua_type(L.s, C.int(index))) == LUA_TTHREAD }
func (L *State) IsUserdata(index int) bool  { return C.lua_isuserdata(L.s, C.int(index)) == 1 }
func (L *State) IsLuaFunction(index int) bool {
    return int(C.lua_type(L.s, C.int(index))) == LUA_TFUNCTION
}

// TO
func (L *State) ToBoolean(index int) bool {
    return int(C.lua_tointegerx(L.s, C.int(index), nil)) == 1
}
func (L *State) ToString(index int) string {
    var size C.size_t
    r := C.lua_tolstring(L.s, C.int(index), &size)
    return C.GoStringN(r, C.int(size))
}
func (L *State) ToBytes(index int) []byte {
    var size C.size_t
    b := C.lua_tolstring(L.s, C.int(index), &size)
    return C.GoBytes(unsafe.Pointer(b), C.int(size))
}
func (L *State) ToInteger(index int) int {
    return int(C.lua_tointegerx(L.s, C.int(index), nil))
}
func (L *State) ToNumber(index int) float64 {
    return float64(C.lua_tonumberx(L.s, C.int(index), nil))
}
func (L *State) ToGoFunction(index int) (f GoFunction) {
    if !L.IsGoFunction(index) {
        return nil
    }
    fid := uint32(C.c_togofunction(L.s, C.int(index)))
    if fid < 0 {
        return nil
    }
    ptr := L.getRegister(fid)
    if fn, ok := ptr.(GoFunction); ok {
        return fn
    }
    return nil
}
func (L *State) ToGoStruct(index int) (f interface{}) {
    if !L.IsGoStruct(index) {
        return nil
    }
    fid := uint32(C.c_togostruct(L.s, C.int(index)))
    if fid < 0 {
        return nil
    }
    return L.registry[fid]
}

// lua_topointer
func (L *State) ToPointer(index int) uintptr {
    return uintptr(C.lua_topointer(L.s, C.int(index)))
}

// lua_touserdata
func (L *State) ToUserdata(index int) unsafe.Pointer {
    return unsafe.Pointer(C.lua_touserdata(L.s, C.int(index)))
}

// lua_xmove
func XMove(from *State, to *State, n int) {
    C.lua_xmove(from.s, to.s, C.int(n))
}

// lua_yield
func (L *State) Yield(nresults int) int {
    return int(C.lua_yieldk(L.s, C.int(nresults), 0, nil))
}

// Push
func (L *State) PushString(str string) {
    Cstr := C.CString(str)
    defer C.free(unsafe.Pointer(Cstr))
    C.lua_pushlstring(L.s, Cstr, C.size_t(len(str)))
}
func (L *State) PushBytes(b []byte) {
    C.lua_pushlstring(L.s, (*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
}
func (L *State) PushInteger(n int64) {
    C.lua_pushinteger(L.s, C.lua_Integer(n))
}
func (L *State) PushNil() {
    C.lua_pushnil(L.s)
}
func (L *State) PushNumber(n float64) {
    C.lua_pushnumber(L.s, C.lua_Number(n))
}
func (L *State) PushValue(index int) {
    C.lua_pushvalue(L.s, C.int(index))
}
func (L *State) PushGoFunction(f GoFunction) {
    id := L.register(f)
    C.c_pushgofunction(L.s, C.uint(id))
}
func (L *State) PushGoStruct(p interface{}) {
    id := L.register(p)
    C.c_pushgostruct(L.s, C.uint(id))
}
func (L *State) PushBoolean(b bool) {
    var bint int
    if b {
        bint = 1
    } else {
        bint = 0
    }
    C.lua_pushboolean(L.s, C.int(bint))
}
func (L *State) PushLightUserdata(ud *interface{}) {
    // push
    C.lua_pushlightuserdata(L.s, unsafe.Pointer(ud))
}
func (L *State) PushGoClosure(f GoFunction) {
    L.PushGoFunction(f)
    C.c_pushcallback(L.s)
}

// Global
func (L *State) RegisterFunction(name string, f GoFunction) {
    L.PushGoFunction(f)
    L.SetGlobal(name)
}
func (L *State) RegisterGoStruct(name string, obj interface{}) {
    L.PushGoStruct(obj)
    L.SetGlobal(name)
}
func (L *State) GC(what, data int) int {
    return int(C.xlua_gc(L.s, C.int(what), C.int(data)))
}
func (L *State) GCCount() (int64, int64) {
    L.registerM.Lock()
    defer L.registerM.Unlock()
    refNum := int64(len(L.registry))
    gcNum := atomic.LoadInt64(&L.gcCount)
    return refNum, gcNum
}

func (L *State) GetGlobal(name string) {
    CName := C.CString(name)
    defer C.free(unsafe.Pointer(CName))
    C.lua_getglobal(L.s, CName)
}
func (L *State) SetGlobal(name string) {
    Cname := C.CString(name)
    defer C.free(unsafe.Pointer(Cname))
    C.lua_setglobal(L.s, Cname)
}
func (L *State) RawGet(index int) {
    C.lua_rawget(L.s, C.int(index))
}
func (L *State) RawGeti(index int, n int) {
    C.lua_rawgeti(L.s, C.int(index), C.longlong(n))
}
func (L *State) RawSet(index int) {
    C.lua_rawset(L.s, C.int(index))
}
func (L *State) RawSeti(index int, n int) {
    C.lua_rawseti(L.s, C.int(index), C.longlong(n))
}

// Table
func (L *State) NewTable() {
    C.lua_createtable(L.s, 0, 0)
}
func (L *State) GetField(index int, k string) {
    Ck := C.CString(k)
    defer C.free(unsafe.Pointer(Ck))
    C.lua_getfield(L.s, C.int(index), Ck)
}
func (L *State) SetTable(n int) {
    C.lua_settable(L.s, C.int(n))
}

//export g_gofunction
func g_gofunction(L *C.lua_State, fid uint32) int {
    L1 := getGoState(L)
    if fid < 0 {
        return 0
    }
    obj := L1.getRegister(fid)
    if obj == nil {
        return 0
    }
    fn, ok := obj.(GoFunction)
    if !ok {
        return 0
    }
    return fn(L1)
}

//export g_gogc
func g_gogc(L *C.lua_State, fid uint32) int {
    L1 := getGoState(L)
    if fid < 0 {
        return 0
    }
    L1.delRegister(fid)
    atomic.AddInt64(&L1.gcCount, 1)
    return 0
}

//export g_getfield
func g_getfield(L *C.lua_State, fid uint32, fieldName *C.char) int {
    L1 := getGoState(L)
    if fid < 0 {
        return 0
    }
    obj := L1.getRegister(fid)
    if obj == nil {
        return 0
    }
    name := C.GoString(fieldName)
    ele := reflect.ValueOf(obj).Elem()
    fval := ele.FieldByName(name)
    if fval.Kind() == reflect.Ptr {
        fval = fval.Elem()
    }

    switch fval.Kind() {
    case reflect.Bool:
        L1.PushBoolean(fval.Bool())
        return 1
    case reflect.String:
        L1.PushString(fval.String())
        return 1
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        L1.PushInteger(fval.Int())
        return 1
    case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
        L1.PushInteger(int64(fval.Uint()))
        return 1
    case reflect.Float32, reflect.Float64:
        L1.PushNumber(fval.Float())
        return 1
    case reflect.Slice:
        L1.PushBytes(fval.Bytes())
        return 1
    default:
        // todo ?????????????????????
        fval = reflect.ValueOf(obj).MethodByName(name)
        if fval.Kind() == reflect.Func {
            // ??????????????????
            L1.PushGoFunction(L1.makeFunc(obj, name, fval))
            return 1
        }
        return 0
    }
}

func (L *State) makeFunc(sender interface{}, funcName string, value reflect.Value) GoFunction {
    return func(L *State) int {
        defer func() {
            if e := recover(); e != nil {
                log.Printf("invoke [%s] error:%v\n", funcName, e)
            }
        }()
        t := value.Type()
        var (
            inArgs  []reflect.Value
            outArgs []reflect.Value
        )

        for i := 0; i < t.NumIn(); i++ {
            idx := i + 2
            luatype := L.Type(idx)
            k := t.In(i).Kind()
            log.Println("????????????", funcName, k)
            switch k {
            case reflect.Bool:
                if luatype != LUA_TNUMBER {
                    return 0
                }
                inArgs = append(inArgs, reflect.ValueOf(L.ToInteger(idx) == 1))
            case reflect.String:
                if luatype != LUA_TSTRING {
                    return 0
                }
                inArgs = append(inArgs, reflect.ValueOf(L.ToString(idx)))
            case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
                if luatype == LUA_TNUMBER {
                    inArgs = append(inArgs, reflect.ValueOf(L.ToInteger(idx)))
                } else if luatype == LUA_TFUNCTION {
                    ref := L.RefX()
                    inArgs = append(inArgs, reflect.ValueOf(ref))
                }
            case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
                if luatype == LUA_TNUMBER {
                    inArgs = append(inArgs, reflect.ValueOf(uint64(L.ToInteger(idx))))
                } else {
                    return 0
                }
            case reflect.Float32, reflect.Float64:
                if luatype != LUA_TNUMBER {
                    return 0
                }
                inArgs = append(inArgs, reflect.ValueOf(L.ToNumber(idx)))
            case reflect.Interface:
                if luatype != LUA_TUSERDATA {
                    return 0
                }
                inArgs = append(inArgs, reflect.ValueOf(L.ToGoStruct(idx)))
            case reflect.Ptr:
                inArgs = append(inArgs, reflect.ValueOf(L.ToGoStruct(idx)))
            case reflect.Slice:
                if luatype != LUA_TSTRING {
                    return 0
                }
                L.ToBoolean(idx)
                inArgs = append(inArgs, reflect.ValueOf(L.ToBytes(idx)))
            default:
                log.Println("??????In?????????", k)
                return 0
            }
        }
        // ??????????????????
        outArgs = value.Call(inArgs)
        n := 0
        //
        for i, fval := range outArgs {
            switch fval.Kind() {
            case reflect.Bool:
                L.PushBoolean(fval.Bool())
                n++
            case reflect.String:
                L.PushString(fval.String())
                n++
            case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
                L.PushInteger(fval.Int())
                n++
            case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
                L.PushInteger(int64(fval.Uint()))
                n++
            case reflect.Float32, reflect.Float64:
                L.PushNumber(fval.Float())
                n++
            case reflect.Slice:
                L.PushBytes(fval.Bytes())
                n++
            case reflect.Interface:
                if fval.Interface() == nil {
                    L.PushNil()
                } else {
                    L.PushGoStruct(fval.Interface())
                }
                n++
            case reflect.Ptr:
                if fval.Interface() == nil {
                    L.PushNil()
                } else {
                    L.PushGoStruct(fval.Interface())
                }
                n++
            default:
                log.Printf("???%d?????????????????????(%v)", i, fval.Kind())
            }
        }
        return n
    }
}

//export g_setfield
func g_setfield(L *C.lua_State, fid uint32, fieldName *C.char) int {
    L1 := getGoState(L)
    if fid < 0 {
        return 0
    }
    obj := L1.getRegister(fid)
    if obj == nil {
        return 0
    }
    name := C.GoString(fieldName)
    vobj := reflect.ValueOf(obj)
    ele := vobj.Elem()
    fval := ele.FieldByName(name)

    if fval.Kind() == reflect.Ptr {
        fval = fval.Elem()
    }
    luatype := int(C.lua_type(L1.s, 3))
    switch fval.Kind() {
    case reflect.Bool:
        if luatype == LUA_TBOOLEAN {
            fval.SetBool(int(C.lua_toboolean(L1.s, 3)) != 0)
            return 1
        }
        return 0
    case reflect.String:
        if luatype == LUA_TSTRING {
            fval.SetString(C.GoString(C.lua_tolstring(L1.s, 3, nil)))
            return 1
        }
        return 0
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        if luatype == LUA_TNUMBER {
            fval.SetInt(int64(C.lua_tointegerx(L1.s, 3, nil)))
            return 1
        }
        return 0
    case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
        if luatype == LUA_TNUMBER {
            fval.SetUint(uint64(C.lua_tointegerx(L1.s, 3, nil)))
            return 1
        }
        return 0
    case reflect.Float32, reflect.Float64:
        if luatype == LUA_TNUMBER {
            fval.SetFloat(float64(C.lua_tonumberx(L1.s, 3, nil)))
            return 1
        }
        return 0
    case reflect.Slice:
        var typeOfBytes = reflect.TypeOf([]byte(nil))
        if luatype == LUA_TSTRING && fval.Type() == typeOfBytes {
            fval.SetBytes(L1.ToBytes(3))
            return 1
        }
        return 0
    default:
        return 0
    }
}
