package telegrambot

import (
	"fmt"
	"os"
	"strings"
)

const (
	KnownUsersFile   = "known_users.txt"
	PendingNamesFile = "pending_names.txt"
)

// UserRecord - структура для хранения пользователя
// Для known_users.txt: id:name:role:username:phone
// Для pending_names.txt: id:name:username:phone

type UserRecord struct {
	ID      int64
	Name    string
	Role    string // только для known_users
	Username string
	Phone   string
}

// --- KNOWN USERS ---

func IsKnownUser(chatID int64) bool {
	if chatID == SystemAdminID {
		return true
	}
	users, _ := LoadKnownUsers()
	_, ok := users[chatID]
	return ok
}

func AddKnownUserFull(user UserRecord) error {
	f, err := os.OpenFile(KnownUsersFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(fmt.Sprintf("%d:%s:%s:%s:%s\n", user.ID, user.Name, user.Role, user.Username, user.Phone))
	return err
}

func RemoveKnownUser(chatID int64) error {
	users, _ := LoadKnownUsers()
	delete(users, chatID)
	return SaveAllKnownUsers(users)
}

func SaveAllKnownUsers(users map[int64]UserRecord) error {
	var lines []string
	for _, u := range users {
		lines = append(lines, fmt.Sprintf("%d:%s:%s:%s:%s", u.ID, u.Name, u.Role, u.Username, u.Phone))
	}
	return os.WriteFile(KnownUsersFile, []byte(strings.Join(lines, "\n")), 0644)
}

func LoadKnownUsers() (map[int64]UserRecord, error) {
	data, err := os.ReadFile(KnownUsersFile)
	if err != nil {
		return map[int64]UserRecord{}, nil
	}
	lines := strings.Split(string(data), "\n")
	users := make(map[int64]UserRecord)
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 5)
		if len(parts) < 5 {
			continue
		}
		var id int64
		fmt.Sscanf(parts[0], "%d", &id)
		users[id] = UserRecord{
			ID:      id,
			Name:    parts[1],
			Role:    parts[2],
			Username: parts[3],
			Phone:   parts[4],
		}
	}
	return users, nil
}

// EnsureSystemAdminInKnownUsers добавляет системного администратора в known_users.txt, если его там нет
func EnsureSystemAdminInKnownUsers() error {
	users, _ := LoadKnownUsers()
	if _, ok := users[SystemAdminID]; !ok {
		sys := UserRecord{
			ID:      SystemAdminID,
			Name:    "Суперадмин",
			Role:    "system_admin",
			Username: "superadmin",
			Phone:   "",
		}
		return AddKnownUserFull(sys)
	}
	return nil
}

// --- PENDING USERS ---

func AddPendingUser(user UserRecord) error {
	f, err := os.OpenFile(PendingNamesFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(fmt.Sprintf("%d:%s:%s:%s\n", user.ID, user.Name, user.Username, user.Phone))
	return err
}

func RemovePendingUser(chatID int64) error {
	users, _ := LoadPendingUsers()
	delete(users, chatID)
	return SaveAllPendingUsers(users)
}

func SaveAllPendingUsers(users map[int64]UserRecord) error {
	var lines []string
	for _, u := range users {
		lines = append(lines, fmt.Sprintf("%d:%s:%s:%s", u.ID, u.Name, u.Username, u.Phone))
	}
	return os.WriteFile(PendingNamesFile, []byte(strings.Join(lines, "\n")), 0644)
}

func LoadPendingUsers() (map[int64]UserRecord, error) {
	data, err := os.ReadFile(PendingNamesFile)
	if err != nil {
		return map[int64]UserRecord{}, nil
	}
	lines := strings.Split(string(data), "\n")
	users := make(map[int64]UserRecord)
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			continue
		}
		var id int64
		fmt.Sscanf(parts[0], "%d", &id)
		users[id] = UserRecord{
			ID:      id,
			Name:    parts[1],
			Username: parts[2],
			Phone:   parts[3],
		}
	}
	return users, nil
} 