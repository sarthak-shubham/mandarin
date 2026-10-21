package store

import "github.com/redis/go-redis/v9"

type Store struct {
	Redis     *redis.Client
	BatchSize int
}

func NewStore(client *redis.Client) *Store {
	s := &Store{
		Redis:     client,
		BatchSize: 1000,
	}
	return s
}

func DeliveryStreamKey() string {
	key := "tamarind:deliveries"
	return key
}

func endpointKey(id string) string {
	key := "endpoint:" + id
	return key
}

func allEndpointsKey() string {
	key := "endpoints:all"
	return key
}

func subscriptionsKey(id string) string {
	key := "subscriptions:" + id
	return key
}

func eventTypeKey(eventType string) string {
	key := "event_type:" + eventType
	return key
}

func eventKey(id string) string {
	key := "event:" + id
	return key
}

func deliveryKey(eventID, endpointID string) string {
	key := "delivery:" + eventID + ":" + endpointID
	return key
}

func eventDeliveriesKey(eventID string) string {
	key := "event_deliveries:" + eventID
	return key
}

func deliveryStreamKey() string {
	key := DeliveryStreamKey()
	return key
}
