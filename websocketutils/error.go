package websocketutils

import "errors"

// ErrConnClosed 表示连接已经关闭。
var ErrConnClosed = errors.New("websocketutils: connection closed")

// ErrNoSuchRoom 表示目标房间不存在。
var ErrNoSuchRoom = errors.New("websocketutils: room not found")

// ErrBufferFull 表示连接发送缓冲已满。
var ErrBufferFull = errors.New("websocketutils: connection buffer full")
