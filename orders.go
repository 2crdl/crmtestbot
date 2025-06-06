package telegrambot

import (
	"encoding/json"
	"os"
)

const OrdersFile = "orders.json"

type Order struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Status     string `json:"status"`
	StartPhoto string `json:"start_photo"`
	EndPhoto   string `json:"end_photo"`
}

func LoadOrders() ([]Order, error) {
	data, err := os.ReadFile(OrdersFile)
	if err != nil {
		return []Order{}, nil
	}
	if len(data) == 0 {
		return []Order{}, nil
	}
	var orders []Order
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func SaveOrders(orders []Order) error {
	data, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(OrdersFile, data, 0644)
}

func AddOrder(order Order) error {
	orders, err := LoadOrders()
	if err != nil {
		return err
	}
	orders = append(orders, order)
	return SaveOrders(orders)
}

func UpdateOrder(order Order) error {
	orders, err := LoadOrders()
	if err != nil {
		return err
	}
	for i, o := range orders {
		if o.ID == order.ID {
			orders[i] = order
			return SaveOrders(orders)
		}
	}
	return nil
}

func GetOrdersByUser(userID int64) ([]Order, error) {
	orders, err := LoadOrders()
	if err != nil {
		return nil, err
	}
	var result []Order
	for _, o := range orders {
		if o.UserID == userID {
			result = append(result, o)
		}
	}
	return result, nil
}

func NextOrderID(orders []Order) int64 {
	var max int64
	for _, o := range orders {
		if o.ID > max {
			max = o.ID
		}
	}
	return max + 1
}
