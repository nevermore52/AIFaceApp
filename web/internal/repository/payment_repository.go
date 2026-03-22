package repository

import (
	"database/sql"
	"time"

	"telegram-ai-face-bot/web/internal/models"
)

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) GetByUserID(userID int64, limit, offset int) ([]*models.Payment, int, error) {
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM completed_payments WHERE telegram_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, telegram_id, username, first_name, last_name, payment_id,
			   category, qty, amount, created_at
		FROM completed_payments WHERE telegram_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payments []*models.Payment
	for rows.Next() {
		p := &models.Payment{}
		if err := rows.Scan(
			&p.ID, &p.TelegramID, &p.Username, &p.FirstName, &p.LastName,
			&p.PaymentID, &p.Category, &p.Qty, &p.Amount, &p.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		payments = append(payments, p)
	}
	return payments, total, rows.Err()
}

func (r *PaymentRepository) GetAll(limit, offset int) ([]*models.Payment, int, error) {
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM completed_payments`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, telegram_id, username, first_name, last_name, payment_id,
			   category, qty, amount, created_at
		FROM completed_payments ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payments []*models.Payment
	for rows.Next() {
		p := &models.Payment{}
		if err := rows.Scan(
			&p.ID, &p.TelegramID, &p.Username, &p.FirstName, &p.LastName,
			&p.PaymentID, &p.Category, &p.Qty, &p.Amount, &p.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		payments = append(payments, p)
	}
	return payments, total, rows.Err()
}

func (r *PaymentRepository) GetStats(since time.Time) (count int, totalAmount float64, err error) {
	query := `SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM completed_payments WHERE created_at > $1`
	err = r.db.QueryRow(query, since).Scan(&count, &totalAmount)
	return
}

func (r *PaymentRepository) GetStatsAll() (count int, totalAmount float64, err error) {
	query := `SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM completed_payments`
	err = r.db.QueryRow(query).Scan(&count, &totalAmount)
	return
}

func (r *PaymentRepository) Create(payment *models.Payment) error {
	query := `
		INSERT INTO completed_payments (telegram_id, username, first_name, last_name, payment_id, category, qty, amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`
	return r.db.QueryRow(query,
		payment.TelegramID, payment.Username, payment.FirstName, payment.LastName,
		payment.PaymentID, payment.Category, payment.Qty, payment.Amount,
	).Scan(&payment.ID, &payment.CreatedAt)
}
