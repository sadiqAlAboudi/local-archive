package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"local-archive/internal/models"

	"golang.org/x/crypto/bcrypt"
)

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if cookie, err := r.Cookie("session_token"); err == nil && cookie.Value != "" {
			userID, exp, mustChange, err := a.db.GetSessionUser(cookie.Value)
			if err == nil && userID > 0 && time.Now().Before(exp) {
				if mustChange {
					http.Redirect(w, r, "/change-credentials", http.StatusSeeOther)
					return
				}
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}
		a.renderTemplate(w, "login.html", models.LoginViewData{
			ShowDefaultNotice: a.db.HasDefaultCredentials(),
		})
		return
	}

	if r.Method == http.MethodPost {
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		user, err := a.db.GetUserByUsername(username)
		if err != nil {
			a.renderTemplate(w, "login.html", models.LoginViewData{
				Error:             "اسم المستخدم أو كلمة المرور غير صحيحة",
				ShowDefaultNotice: a.db.HasDefaultCredentials(),
			})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
			a.renderTemplate(w, "login.html", models.LoginViewData{
				Error:             "اسم المستخدم أو كلمة المرور غير صحيحة",
				ShowDefaultNotice: a.db.HasDefaultCredentials(),
			})
			return
		}

		tokenBytes := make([]byte, 32)
		rand.Read(tokenBytes)
		token := hex.EncodeToString(tokenBytes)
		expiresAt := time.Now().Add(30 * 24 * time.Hour)

		if err := a.db.CreateSession(token, user.ID, expiresAt); err != nil {
			http.Error(w, "فشل إنشاء الجلسة", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Path:     "/",
			Expires:  expiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		if user.MustChangeCredentials == 1 {
			http.Redirect(w, r, "/change-credentials", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (a *App) handleChangeCredentials(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID, _, _, err := a.db.GetSessionUser(cookie.Value)
	if err != nil || userID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := a.db.GetUserByID(userID)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		a.renderTemplate(w, "change_credentials.html", models.ChangeCredsViewData{
			CurrentUsername: user.Username,
		})
		return
	}

	if r.Method == http.MethodPost {
		newUsername := strings.TrimSpace(r.FormValue("username"))
		newPassword := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if newUsername == "" || newPassword == "" {
			a.renderTemplate(w, "change_credentials.html", models.ChangeCredsViewData{
				CurrentUsername: user.Username,
				Error:           "يجب إدخال اسم المستخدم وكلمة المرور الجديدة",
			})
			return
		}

		if newPassword != confirmPassword {
			a.renderTemplate(w, "change_credentials.html", models.ChangeCredsViewData{
				CurrentUsername: newUsername,
				Error:           "كلمة المرور وتأكيد كلمة المرور غير متطابقين",
			})
			return
		}

		if len(newPassword) < 4 {
			a.renderTemplate(w, "change_credentials.html", models.ChangeCredsViewData{
				CurrentUsername: newUsername,
				Error:           "يجب أن تتكون كلمة المرور من 4 خانات على الأقل",
			})
			return
		}

		taken, err := a.db.UsernameExists(newUsername, userID)
		if err != nil || taken {
			a.renderTemplate(w, "change_credentials.html", models.ChangeCredsViewData{
				CurrentUsername: newUsername,
				Error:           "اسم المستخدم هذا مستخدم بالفعل، اختر اسماً آخر",
			})
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "فشل تشفير كلمة المرور", http.StatusInternalServerError)
			return
		}

		if err := a.db.UpdateUserCredentials(userID, newUsername, string(hash)); err != nil {
			a.renderTemplate(w, "change_credentials.html", models.ChangeCredsViewData{
				CurrentUsername: newUsername,
				Error:           "تعذر حفظ التغييرات: " + err.Error(),
			})
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session_token"); err == nil && cookie.Value != "" {
		_ = a.db.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
