package telegrambot

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	SystemAdminID int64 = 6398798394
)

var (
	adminID int64
	apiToken string
	feedbackWaiting = make(map[int64]bool)
	userRoles = make(map[int64]string) // chatID -> role
	pendingRoleChoice = make(map[int64]bool)
	pendingApproveUser = make(map[int64]int64) // admin chatID -> user chatID
)

var forbiddenNames = map[string]bool{
	"📦 Мои заказы": true,
	"💬 Связь с админом": true,
	"👥 Пользователи": true,
	"📱 Отправить контакт": true,
	"🛠 Сообщить администратору": true,
	"❌ Отмена действия": true,
	"✅ Активные": true,
	"⏳ Ожидающие": true,
	"✅ Принять": true,
	"❌ Отклонить": true,
	"🗑 Удалить": true,
	"Администратор": true,
	"Обувщик": true,
	"Реставратор": true,
	"Химчистер": true,
}

func getRole(chatID int64) string {
	if chatID == SystemAdminID {
		return "system_admin"
	}
	if userRoles[chatID] != "" {
		return userRoles[chatID]
	}
	if chatID == adminID {
		return "admin"
	}
	if IsKnownUser(chatID) {
		return "user"
	}
	return "guest"
}

func setRole(chatID int64, role string) {
	userRoles[chatID] = role
}

// --- ВАЛИДАЦИЯ ИМЕНИ ---
func isValidName(name string) bool {
	if forbiddenNames[name] {
		return false
	}
	// Только буквы, цифры, пробелы, кириллица/латиница
	matched, _ := regexp.MatchString(`^[a-zA-Zа-яА-ЯёЁ0-9 ]{2,32}$`, name)
	return matched
}

func RunBot(token string, admin int64) {
	apiToken = token
	adminID = admin
	EnsureSystemAdminInKnownUsers() // Гарантируем наличие суперадмина в базе
	setRole(SystemAdminID, "system_admin")
	setRole(adminID, "admin")
	bot, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Загружаем runtime-данные пользователей
	knownUsers, _ := LoadKnownUsers()
	pendingUsers, _ := LoadPendingUsers()
	for id, u := range knownUsers {
		userRoles[id] = u.Role
	}

	pending := make(map[int64]string) // временно для имени

	for update := range updates {
		if update.Message != nil {
			chatID := update.Message.Chat.ID
			role := getRole(chatID)
			isAdmin := role == "admin" || role == "system_admin"
			isSystemAdmin := role == "system_admin"

			// --- Проверка выбора роли после принятия заявки ---
			if pendingRoleChoice[chatID] {
				if (isSystemAdmin && (update.Message.Text == "Администратор" || update.Message.Text == "Обувщик" || update.Message.Text == "Реставратор" || update.Message.Text == "Химчистер")) ||
					(!isSystemAdmin && (update.Message.Text == "Обувщик" || update.Message.Text == "Реставратор" || update.Message.Text == "Химчистер")) {
					uid := pendingApproveUser[chatID]
					pendingUser, ok := pendingUsers[uid]
					if !ok {
						bot.Send(tgbotapi.NewMessage(chatID, "Ошибка: пользователь не найден в ожидании."))
						continue
					}
					// Добавляем в known_users.txt
					newUser := UserRecord{
						ID:      pendingUser.ID,
						Name:    pendingUser.Name,
						Role:    update.Message.Text,
						Username: pendingUser.Username,
						Phone:   pendingUser.Phone,
					}
					AddKnownUserFull(newUser)
					userRoles[uid] = update.Message.Text
					RemovePendingUser(uid)
					msg := tgbotapi.NewMessage(uid, "Ваша заявка одобрена! Ваша роль: "+update.Message.Text)
					if update.Message.Text != "Администратор" {
						msg.ReplyMarkup = userMenu()
					}
					bot.Send(msg)
					// Уведомление админу о том, что пользователь одобрен
					if chatID != SystemAdminID {
						adminMsg := tgbotapi.NewMessage(chatID, "Пользователь одобрен и назначена роль: "+update.Message.Text)
						adminMsg.ReplyMarkup = usersMenu()
						bot.Send(adminMsg)
					} else {
						// Для суперадмина просто обновляем меню
						adminMsg := tgbotapi.NewMessage(chatID, "Пользователь одобрен и назначена роль: "+update.Message.Text)
						adminMsg.ReplyMarkup = usersMenu()
						bot.Send(adminMsg)
					}
					delete(pendingRoleChoice, chatID)
					delete(pendingApproveUser, chatID)
					continue
				}
				msg := tgbotapi.NewMessage(chatID, "Выберите роль из предложенных кнопок!")
				msg.ReplyMarkup = roleChoiceMenu(isSystemAdmin)
				bot.Send(msg)
				continue
			}

			// --- Обработка команды /start ---
			if update.Message.IsCommand() && update.Message.Command() == "start" {
				if isAdmin {
					msg := tgbotapi.NewMessage(chatID, "Вы в админ-панели.")
					msg.ReplyMarkup = adminMenu(isSystemAdmin)
					bot.Send(msg)
					continue
				}
				if IsKnownUser(chatID) {
					msg := tgbotapi.NewMessage(chatID, "Вы уже зарегистрированы.")
					msg.ReplyMarkup = userMenu()
					bot.Send(msg)
					continue
				}
				regKb := tgbotapi.NewReplyKeyboard(
					tgbotapi.NewKeyboardButtonRow(
						tgbotapi.NewKeyboardButtonContact("📱 Отправить контакт"),
					),
					tgbotapi.NewKeyboardButtonRow(
						tgbotapi.NewKeyboardButton("🛠 Сообщить администратору"),
					),
				)
				msg := tgbotapi.NewMessage(chatID, "Пожалуйста, введите своё имя для регистрации:")
				msg.ReplyMarkup = regKb
				bot.Send(msg)
				continue
			}

			// --- Проверка имени ---
			if !IsKnownUser(chatID) && update.Message.Text != "" && pending[chatID] == "" {
				if !isValidName(update.Message.Text) {
					msg := tgbotapi.NewMessage(chatID, "Это имя недопустимо. Пожалуйста, введите другое имя (только буквы, цифры, пробелы, 2-32 символа).")
					bot.Send(msg)
					continue
				}
				pending[chatID] = update.Message.Text
				msg := tgbotapi.NewMessage(chatID, "Теперь нажмите кнопку ниже, чтобы поделиться своим номером телефона:")
				msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
					tgbotapi.NewKeyboardButtonRow(
						tgbotapi.NewKeyboardButtonContact("📱 Отправить контакт"),
					),
				)
				bot.Send(msg)
				continue
			}

			// --- Регистрация ---
			if !IsKnownUser(chatID) {
				if update.Message.Contact != nil {
					name, ok := pending[chatID]
					if !ok {
						msg := tgbotapi.NewMessage(chatID, "Сначала введите своё имя для регистрации:")
						bot.Send(msg)
						continue
					}
					username := update.Message.From.UserName
					if username == "" {
						username = "без username"
					}
					phone := update.Message.Contact.PhoneNumber
					pendingUser := UserRecord{
						ID:      chatID,
						Name:    name,
						Username: username,
						Phone:   phone,
					}
					AddPendingUser(pendingUser)
					pendingUsers[chatID] = pendingUser
					// Уведомление админу
					if chatID != SystemAdminID { // Не отправлять заявку самому себе
						text := fmt.Sprintf("Новый пользователь @%s ожидает подтверждения", username)
						if SystemAdminID != adminID {
							bot.Send(tgbotapi.NewMessage(SystemAdminID, text))
						}
						bot.Send(tgbotapi.NewMessage(adminID, text))
					}
					msg := tgbotapi.NewMessage(chatID, "Спасибо! Заявка отправлена администратору. Ожидайте решения.")
					bot.Send(msg)
					continue
				}
			}

			// --- Меню пользователя ---
			if role == "user" || role == "Обувщик" || role == "Реставратор" || role == "Химчистер" {
				if update.Message.Text == "📦 Мои заказы" {
					msg := tgbotapi.NewMessage(chatID, "Ваши заказы (заглушка)")
					msg.ReplyMarkup = userMenu()
					bot.Send(msg)
					continue
				}
				if update.Message.Text == "💬 Связь с админом" {
					feedbackWaiting[chatID] = true
					cancelKb := tgbotapi.NewReplyKeyboard(
						tgbotapi.NewKeyboardButtonRow(
							tgbotapi.NewKeyboardButton("❌ Отмена действия"),
						),
					)
					msg := tgbotapi.NewMessage(chatID, "Пожалуйста, введите ваше сообщение для администратора:")
					msg.ReplyMarkup = cancelKb
					bot.Send(msg)
					continue
				}
				msg := tgbotapi.NewMessage(chatID, "Выберите действие из меню:")
				msg.ReplyMarkup = userMenu()
				bot.Send(msg)
				continue
			}

			// --- Ожидание обратной связи ---
			if feedbackWaiting[chatID] {
				if update.Message.Text == "❌ Отмена действия" {
					feedbackWaiting[chatID] = false
					msg := tgbotapi.NewMessage(chatID, "Действие отменено. Выберите действие из меню:")
					msg.ReplyMarkup = userMenu()
					bot.Send(msg)
					continue
				}
				// Отправляем сообщение админу и системному админу
				text := fmt.Sprintf("💬 Обратная связь от пользователя %d: %s", chatID, update.Message.Text)
				if SystemAdminID != adminID {
					bot.Send(tgbotapi.NewMessage(SystemAdminID, text))
				}
				bot.Send(tgbotapi.NewMessage(adminID, text))
				feedbackWaiting[chatID] = false
				msg := tgbotapi.NewMessage(chatID, "Ваше сообщение отправлено администратору. Выберите действие из меню:")
				msg.ReplyMarkup = userMenu()
				bot.Send(msg)
				continue
			}

			// --- Меню админа ---
			if isAdmin {
				if update.Message.Text == "👥 Пользователи" {
					msg := tgbotapi.NewMessage(chatID, "Выберите категорию:")
					msg.ReplyMarkup = usersMenu()
					bot.Send(msg)
					continue
				}
				if update.Message.Text == "✅ Активные" {
					knownUsers, _ = LoadKnownUsers()
					if len(knownUsers) == 0 {
						msg := tgbotapi.NewMessage(chatID, "Нет активных пользователей.")
						msg.ReplyMarkup = usersMenu()
						bot.Send(msg)
						continue
					}
					rows := [][]tgbotapi.KeyboardButton{}
					for _, u := range knownUsers {
						if u.ID == chatID && (u.Role == "admin" || u.Role == "system_admin") {
							continue // не показываем себя
						}
						if u.Role == "admin" && !isSystemAdmin {
							continue // обычный админ не видит других админов
						}
						rows = append(rows, tgbotapi.NewKeyboardButtonRow(
							tgbotapi.NewKeyboardButton(fmt.Sprintf("%s (%s) 🗑", u.Name, u.Role)),
						))
					}
					menu := tgbotapi.NewReplyKeyboard(rows...)
					menu.Keyboard = append(menu.Keyboard, tgbotapi.NewKeyboardButtonRow(
						tgbotapi.NewKeyboardButton("⬅️ Назад"),
					))
					msg := tgbotapi.NewMessage(chatID, "Активные пользователи:")
					msg.ReplyMarkup = menu
					bot.Send(msg)
					continue
				}
				if update.Message.Text == "⏳ Ожидающие" {
					pendingUsers, _ = LoadPendingUsers()
					if len(pendingUsers) == 0 {
						msg := tgbotapi.NewMessage(chatID, "Нет ожидающих пользователей.")
						msg.ReplyMarkup = usersMenu()
						bot.Send(msg)
						continue
					}
					rows := [][]tgbotapi.KeyboardButton{}
					for _, u := range pendingUsers {
						rows = append(rows, tgbotapi.NewKeyboardButtonRow(
							tgbotapi.NewKeyboardButton(fmt.Sprintf("%s (%d)", u.Name, u.ID)),
						))
					}
					menu := tgbotapi.NewReplyKeyboard(rows...)
					menu.Keyboard = append(menu.Keyboard, tgbotapi.NewKeyboardButtonRow(
						tgbotapi.NewKeyboardButton("⬅️ Назад"),
					))
					msg := tgbotapi.NewMessage(chatID, "Ожидающие пользователи:")
					msg.ReplyMarkup = menu
					bot.Send(msg)
					continue
				}
				if update.Message.Text == "⬅️ Назад" {
					msg := tgbotapi.NewMessage(chatID, "Выберите категорию:")
					msg.ReplyMarkup = usersMenu()
					bot.Send(msg)
					continue
				}
				// --- Детальный просмотр пользователя и удаление ---
				if strings.Contains(update.Message.Text, "(") && strings.Contains(update.Message.Text, ")") {
					// Для активных: "Имя (роль) 🗑"; для ожидающих: "Имя (id)"
					if strings.Contains(update.Message.Text, "Администратор") || strings.Contains(update.Message.Text, "system_admin") {
						msg := tgbotapi.NewMessage(chatID, "Действия с этим пользователем недоступны.")
						bot.Send(msg)
						continue
					}
					if strings.HasSuffix(update.Message.Text, "🗑") {
						// Удаление пользователя
						parts := strings.Split(update.Message.Text, "(")
						rolePart := strings.TrimSuffix(strings.TrimSpace(parts[1]), ") 🗑")
						// Находим пользователя по имени и роли
						var toDeleteID int64 = 0
						for id, u := range knownUsers {
							if u.Name == parts[0] && u.Role == rolePart {
								toDeleteID = id
								break
							}
						}
						if toDeleteID != 0 {
							RemoveKnownUser(toDeleteID)
							delete(userRoles, toDeleteID)
							msg := tgbotapi.NewMessage(chatID, "Пользователь удалён.")
							bot.Send(msg)
							knownUsers, _ = LoadKnownUsers()
						} else {
							msg := tgbotapi.NewMessage(chatID, "Пользователь не найден.")
							bot.Send(msg)
						}
						continue
					}
					// Ожидающие: "Имя (id)"
					if strings.Contains(update.Message.Text, "Ожидающие") {
						continue
					}
					parts := strings.Split(update.Message.Text, "(")
					idStr := strings.TrimSuffix(parts[1], ")")
					var uid int64
					fmt.Sscanf(idStr, "%d", &uid)
					pendingRoleChoice[chatID] = true
					pendingApproveUser[chatID] = uid
					msg := tgbotapi.NewMessage(chatID, "Выберите роль для пользователя:")
					msg.ReplyMarkup = roleChoiceMenu(isSystemAdmin)
					bot.Send(msg)
					continue
				}
			}
		}
		if update.CallbackQuery != nil {
			// ... (оставить пустым или реализовать по необходимости)
		}
	}
}

func userMenu() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📦 Мои заказы"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("💬 Связь с админом"),
		),
	)
}

func adminMenu(isSystemAdmin bool) tgbotapi.ReplyKeyboardMarkup {
	menu := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("👥 Пользователи"),
		),
	)
	return menu
}

func usersMenu() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✅ Активные"),
			tgbotapi.NewKeyboardButton("⏳ Ожидающие"),
		),
	)
}

func roleChoiceMenu(isSystemAdmin bool) tgbotapi.ReplyKeyboardMarkup {
	row := []tgbotapi.KeyboardButton{
		tgbotapi.NewKeyboardButton("Обувщик"),
		tgbotapi.NewKeyboardButton("Реставратор"),
		tgbotapi.NewKeyboardButton("Химчистер"),
	}
	if isSystemAdmin {
		row = append(row, tgbotapi.NewKeyboardButton("Администратор"))
	}
	return tgbotapi.NewReplyKeyboard(row)
} 