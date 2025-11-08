package queue

import (
	"go_chat_backend/platform/cache"
)

type MessageQueueService struct {
	MQ cache.MessageQueue
}

func NewMessageService(mq cache.MessageQueue) cache.MessageQueue {
	return &MessageQueueService{MQ: mq}
}
func (mq *MessageQueueService) PushToQueue(queueName string, value interface{}) error {
	return mq.MQ.PushToQueue(queueName, value)
}
func (mq *MessageQueueService) PopFromQueue(queueName string) (interface{}, error) {
	return mq.MQ.PopFromQueue(queueName)
}
