package product

import (
	"context"
	"fmt"
	"go-pet-shop/internal/models"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
)

//go:generate go run github.com/vektra/mockery/v2@v2.53.7 --name=Products
type Products interface {
	GetAllProducts(ctx context.Context) ([]models.Product, error)
	CreateProduct(ctx context.Context, product models.Product) (int, error)
	DeleteProduct(ctx context.Context, id int) error
	UpdateProduct(ctx context.Context, product models.Product) error
}

type Handler struct {
	log     *slog.Logger
	storage Products
}

func New(log *slog.Logger, storage Products) *Handler {
	return &Handler{
		log:     log,
		storage: storage,
	}
}
func (h *Handler) GetAllProducts(w http.ResponseWriter, r *http.Request) {
	const fn = "handlers.products.GetAllProducts"
	log := h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	items, err := h.storage.GetAllProducts(r.Context())

	if err != nil {
		log.Error("failed to get products", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error":   "Internal server error",
			"message": "Failed to retrieve products",
		})
		return
	}

	log.Info("Retrieved products successfully",
		slog.String("url", r.URL.String()),
		slog.Int("count", len(items)),
	)

	render.JSON(w, r, items)
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	const fn = "handlers.products.CreateProduct"

	log := h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	log.Info("Creating new product", slog.String("url", r.URL.String()))

	var product models.Product
	if err := render.DecodeJSON(r.Body, &product); err != nil {
		log.Error("failed to decode request body", slog.Any("error", err))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Invalid JSON payload",
		})
		return
	}

	// Валидация
	if product.Name == "" {
		log.Error("product name is empty")
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product name is required",
		})
		return
	}

	if product.Price < 0 {
		log.Error("product price is negative", slog.Float64("price", product.Price))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product price cannot be negative",
		})
		return
	}

	if product.Stock < 0 {
		log.Error("product stock is negative", slog.Int("stock", product.Stock))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product stock cannot be negative",
		})
		return
	}

	// Создаем продукт
	productID, err := h.storage.CreateProduct(r.Context(), product)
	if err != nil {
		log.Error("failed to create product", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error":   "Internal server error",
			"message": "Failed to create product",
		})
		return
	}

	log.Info("Product created successfully",
		slog.Int("id", productID),
		slog.String("name", product.Name),
		slog.String("url", r.URL.String()),
	)

	// Возвращаем созданный продукт с его ID
	product.ID = productID
	render.JSON(w, r, map[string]interface{}{
		"status":  "Product created successfully",
		"id":      productID,
		"product": product,
	})
}

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	const fn = "handlers.products.DeleteProduct"

	log := h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	log.Info("Deleting product", slog.String("url", r.URL.String()))

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		log.Error("empty id")
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product ID is required",
		})
		return
	}

	// Конвертируем ID в int
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error("invalid id format", slog.Any("error", err), slog.String("id", idStr))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product ID must be a number",
		})
		return
	}

	// Удаляем продукт
	if err := h.storage.DeleteProduct(r.Context(), id); err != nil {
		// Проверяем, является ли ошибка "не найдено" с помощью strings.Contains
		if strings.Contains(strings.ToLower(err.Error()), "not found") ||
			strings.Contains(strings.ToLower(err.Error()), "no rows") ||
			strings.Contains(strings.ToLower(err.Error()), "rows affected: 0") {
			log.Warn("product not found for deletion", slog.Int("id", id))
			w.WriteHeader(http.StatusNotFound)
			render.JSON(w, r, map[string]interface{}{
				"error":   "Not found",
				"message": fmt.Sprintf("Product with ID %d does not exist", id),
				"id":      id,
			})
			return
		}

		log.Error("failed to delete product", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error":   "Internal server error",
			"message": "Failed to delete product",
		})
		return
	}

	log.Info("Deleted product successfully",
		slog.Int("id", id),
		slog.String("url", r.URL.String()),
	)

	render.JSON(w, r, map[string]interface{}{
		"status":  "Product deleted successfully",
		"id":      id,
		"message": fmt.Sprintf("Product with ID %d has been deleted", id),
	})
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	const fn = "handlers.products.UpdateProduct"

	log := h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	log.Info("Updating product", slog.String("url", r.URL.String()))

	// Получаем ID из URL
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		log.Error("empty id in URL")
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product ID is required",
		})
		return
	}

	// Конвертируем ID в int
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error("invalid id format", slog.Any("error", err), slog.String("id", idStr))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product ID must be a number",
		})
		return
	}

	// Декодируем тело запроса
	var product models.Product
	if err := render.DecodeJSON(r.Body, &product); err != nil {
		log.Error("failed to decode request body", slog.Any("error", err))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Invalid JSON payload",
		})
		return
	}

	// Устанавливаем ID из URL (переопределяем ID из тела, если он там был)
	product.ID = id

	// Валидация
	if product.Name == "" {
		log.Error("product name is empty")
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product name is required",
		})
		return
	}

	if product.Price < 0 {
		log.Error("product price is negative", slog.Float64("price", product.Price))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product price cannot be negative",
		})
		return
	}

	if product.Stock < 0 {
		log.Error("product stock is negative", slog.Int("stock", product.Stock))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product stock cannot be negative",
		})
		return
	}

	// Обновляем продукт
	if err := h.storage.UpdateProduct(r.Context(), product); err != nil {
		// Проверяем, является ли ошибка "не найдено" с помощью strings.Contains
		if strings.Contains(strings.ToLower(err.Error()), "not found") ||
			strings.Contains(strings.ToLower(err.Error()), "no rows") ||
			strings.Contains(strings.ToLower(err.Error()), "rows affected: 0") {
			log.Warn("product not found for update", slog.Int("id", id))
			w.WriteHeader(http.StatusNotFound)
			render.JSON(w, r, map[string]interface{}{
				"error":   "Not found",
				"message": fmt.Sprintf("Product with ID %d does not exist", id),
				"id":      id,
			})
			return
		}

		log.Error("failed to update product", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error":   "Internal server error",
			"message": "Failed to update product",
		})
		return
	}

	log.Info("Product updated successfully",
		slog.Int("id", id),
		slog.String("name", product.Name),
		slog.String("url", r.URL.String()),
	)

	// Возвращаем обновленный продукт
	render.JSON(w, r, map[string]interface{}{
		"status":  "Product updated successfully",
		"id":      id,
		"product": product,
	})
}
