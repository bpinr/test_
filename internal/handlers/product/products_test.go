package product

import (
	"context"
	"encoding/json"
	"errors"
	"go-pet-shop/internal/handlers/product/mocks"
	"go-pet-shop/internal/models"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/mock"
)

// withURLParam кладёт параметр пути (например, id) в контекст запроса,
// как это сделал бы роутер chi. Без него chi.URLParam вернёт пустую строку.
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// decodeBody разбирает JSON-ответ в map, чтобы проверять поля тела.
func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return body
}

// Мок создаётся через mocks.NewProducts(t): в конце теста он сам проверит,
// что все ожидаемые вызовы были, а неожиданный вызов сразу уронит тест.

// =======================
// Get All Products
// =======================

func TestGetAllProducts_Success(t *testing.T) {
	// Мокаем storage — он вернёт один продукт.
	m := mocks.NewProducts(t)
	m.On("GetAllProducts", mock.Anything).
		Return([]models.Product{{ID: 1, Name: "Dog Food"}}, nil)

	// Создаем HTTP-запрос GET /products
	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	w := httptest.NewRecorder()

	// Создаем хендлер с мок-хранилищем
	handler := New(slog.Default(), m)

	// Вызываем метод GetAllProducts, который является http.HandlerFunc
	handler.GetAllProducts(w, req)

	// Проверяем HTTP-код
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Проверяем тело: должен вернуться тот же продукт, что отдал мок
	var products []models.Product
	if err := json.NewDecoder(w.Body).Decode(&products); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if len(products) != 1 || products[0].ID != 1 || products[0].Name != "Dog Food" {
		t.Fatalf("unexpected products: %+v", products)
	}
}

func TestGetAllProducts_Error(t *testing.T) {
	// Мокаем storage — он будет возвращать ошибку
	m := mocks.NewProducts(t)
	m.On("GetAllProducts", mock.Anything).Return(nil, errors.New("DB error"))

	// Создаем запрос
	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.GetAllProducts(w, req)

	// Ожидаем HTTP 500
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Failed to retrieve products" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

// =======================
// Create Product
// =======================

func TestCreateProduct_Success(t *testing.T) {
	m := mocks.NewProducts(t)
	// Хендлер должен передать в storage ровно то, что пришло в JSON
	m.On("CreateProduct", mock.Anything, models.Product{Name: "Cat Toy", Price: 9.99, Stock: 5}).
		Return(42, nil)

	req := httptest.NewRequest(http.MethodPost, "/products",
		strings.NewReader(`{"Name":"Cat Toy","Price":9.99,"Stock":5}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateProduct(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["status"] != "Product created successfully" {
		t.Fatalf("unexpected status: %v", body["status"])
	}
	// JSON-числа декодируются в map как float64
	if body["id"] != float64(42) {
		t.Fatalf("expected id 42, got %v", body["id"])
	}
}

func TestCreateProduct_BadRequest(t *testing.T) {
	// Ожиданий нет: если хендлер дойдёт до storage, мок уронит тест
	m := mocks.NewProducts(t)

	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{bad json`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Invalid JSON payload" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestCreateProduct_Fail(t *testing.T) {
	m := mocks.NewProducts(t)
	m.On("CreateProduct", mock.Anything, mock.Anything).Return(0, errors.New("DB error"))

	req := httptest.NewRequest(http.MethodPost, "/products",
		strings.NewReader(`{"Name":"Cat Toy","Price":9.99,"Stock":5}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateProduct(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Failed to create product" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

// =======================
// Update Product
// =======================

func TestUpdateProduct_Success(t *testing.T) {
	m := mocks.NewProducts(t)
	// ID должен прийти из URL, а не из тела
	m.On("UpdateProduct", mock.Anything, models.Product{ID: 7, Name: "Bird Cage", Price: 50, Stock: 2}).
		Return(nil)

	req := httptest.NewRequest(http.MethodPut, "/products/7",
		strings.NewReader(`{"Name":"Bird Cage","Price":50,"Stock":2}`))
	req = withURLParam(req, "id", "7")
	w := httptest.NewRecorder()

	New(slog.Default(), m).UpdateProduct(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["status"] != "Product updated successfully" {
		t.Fatalf("unexpected status: %v", body["status"])
	}
	if body["id"] != float64(7) {
		t.Fatalf("expected id 7, got %v", body["id"])
	}
}

func TestUpdateProduct_BadRequest(t *testing.T) {
	m := mocks.NewProducts(t)

	req := httptest.NewRequest(http.MethodPut, "/products/7", strings.NewReader(`{bad json`))
	req = withURLParam(req, "id", "7")
	w := httptest.NewRecorder()

	New(slog.Default(), m).UpdateProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Invalid JSON payload" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestUpdateProduct_Fail(t *testing.T) {
	m := mocks.NewProducts(t)
	// Ошибка без "not found", иначе хендлер ответит 404
	m.On("UpdateProduct", mock.Anything, mock.Anything).Return(errors.New("DB error"))

	req := httptest.NewRequest(http.MethodPut, "/products/7",
		strings.NewReader(`{"Name":"Bird Cage","Price":50,"Stock":2}`))
	req = withURLParam(req, "id", "7")
	w := httptest.NewRecorder()

	New(slog.Default(), m).UpdateProduct(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Failed to update product" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

// =======================
// Delete Product
// =======================

func TestDeleteProduct_Success(t *testing.T) {
	m := mocks.NewProducts(t)
	m.On("DeleteProduct", mock.Anything, 3).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/products/3", nil)
	req = withURLParam(req, "id", "3")
	w := httptest.NewRecorder()

	New(slog.Default(), m).DeleteProduct(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["status"] != "Product deleted successfully" {
		t.Fatalf("unexpected status: %v", body["status"])
	}
	if body["id"] != float64(3) {
		t.Fatalf("expected id 3, got %v", body["id"])
	}
}

func TestDeleteProduct_BadRequest(t *testing.T) {
	m := mocks.NewProducts(t)

	// id не кладём в контекст — chi.URLParam вернёт пустую строку
	req := httptest.NewRequest(http.MethodDelete, "/products/", nil)
	w := httptest.NewRecorder()

	New(slog.Default(), m).DeleteProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Product ID is required" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestDeleteProduct_Fail(t *testing.T) {
	m := mocks.NewProducts(t)
	m.On("DeleteProduct", mock.Anything, 3).Return(errors.New("DB error"))

	req := httptest.NewRequest(http.MethodDelete, "/products/3", nil)
	req = withURLParam(req, "id", "3")
	w := httptest.NewRecorder()

	New(slog.Default(), m).DeleteProduct(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Failed to delete product" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}
