package main

import "github.com/redis/go-redis/v9"

type Store struct {
	redis *redis.Client
}

func NewStore(client *redis.Client) *Store {
	return &Store{redis: client}
}

func endpointKey(id string) string                  { return "endpoint:" + id }
func allEndpointsKey() string                       { return "endpoints:all" }
func subscriptionsKey(id string) string             { return "subscriptions:" + id }
func eventTypeKey(eventType string) string          { return "event_type:" + eventType }
func eventKey(id string) string                     { return "event:" + id }
func deliveryKey(eventID, endpointID string) string { return "delivery:" + eventID + ":" + endpointID }
func eventDeliveriesKey(eventID string) string      { return "event_deliveries:" + eventID }
func deliveryStreamKey() string                     { return "mochi:deliveries" }
