package main

import (
	"container/list"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
	"sync"
)

type ClientOption struct {
	apiId            int
	apiHash          string
	botToken         string
	onMessage        string
	onChannelMessage string
	userId           int64
	channelId        int64
	saveDir          string
	logger           *zap.Logger
	dialer           dcs.DialFunc
	threadNum        int
}

type DownloadTask struct {
	entities   tg.Entities
	newMessage message.AnswerableMessageUpdate
	document   *tg.Document
	photo      *tg.Photo
	fineName   string
	retryNum   int
}

type AtomicInt struct {
	Lock  sync.Mutex
	value int
}

func (atomicInt *AtomicInt) Value() int {
	defer atomicInt.Lock.Unlock()
	atomicInt.Lock.Lock()
	return atomicInt.value
}
func (atomicInt *AtomicInt) Higher() int {
	defer atomicInt.Lock.Unlock()
	atomicInt.Lock.Lock()
	atomicInt.value += 1
	return atomicInt.value
}
func (atomicInt *AtomicInt) Lower() int {
	defer atomicInt.Lock.Unlock()
	atomicInt.Lock.Lock()
	atomicInt.value -= 1
	return atomicInt.value
}

type Queue struct {
	List list.List
	Lock sync.Mutex
}

func (queue *Queue) Push(a *DownloadTask) {
	defer queue.Lock.Unlock()
	queue.Lock.Lock()
	queue.List.PushFront(a)
}
func (queue *Queue) Pop() *DownloadTask {
	defer queue.Lock.Unlock()
	queue.Lock.Lock()
	e := queue.List.Back()
	if e != nil {
		queue.List.Remove(e)
		return e.Value.(*DownloadTask)
	}
	return nil
}
func (queue *Queue) Len() int {
	defer queue.Lock.Unlock()
	queue.Lock.Lock()
	length := queue.List.Len()
	return length
}
