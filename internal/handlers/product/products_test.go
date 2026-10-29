package product

import (
	"context"
	"encoding/json"
	"errors"
	"go-pet-shop/internal/models"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi"
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

// =======================
// Get All Products
// =======================

func TestGetAllProducts_Success(t *testing.T) {
	// Мокаем storage — он вернёт один продукт.
	mock := &ProductsMock{
		GetAllProductsFunc: func(ctx context.Context) ([]models.Product, error) {
			return []models.Product{
				{ID: 1, Name: "Dog Food"},
			}, nil
		},
	}

	// Создаем HTTP-запрос GET /products
	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	w := httptest.NewRecorder()

	// Создаем хендлер с мок-хранилищем
	handler := New(slog.Default(), mock)

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
	mock := &ProductsMock{
		GetAllProductsFunc: func(ctx context.Context) ([]models.Product, error) {
			return nil, errors.New("DB error")
		},
	}

	// Создаем запрос
	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	w := httptest.NewRecorder()

	handler := New(slog.Default(), mock)
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
	var got models.Product
	mock := &ProductsMock{
		CreateProductFunc: func(ctx context.Context, product models.Product) (int, error) {
			got = product
			return 42, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/products",
		strings.NewReader(`{"Name":"Cat Toy","Price":9.99,"Stock":5}`))
	w := httptest.NewRecorder()

	New(slog.Default(), mock).CreateProduct(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Хендлер должен передать в storage то, что пришло в JSON
	if got.Name != "Cat Toy" || got.Price != 9.99 || got.Stock != 5 {
		t.Fatalf("storage got unexpected product: %+v", got)
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
	mock := &ProductsMock{
		CreateProductFunc: func(ctx context.Context, product models.Product) (int, error) {
			t.Fatal("storage must not be called on invalid JSON")
			return 0, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{bad json`))
	w := httptest.NewRecorder()

	New(slog.Default(), mock).CreateProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Invalid JSON payload" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestCreateProduct_Fail(t *testing.T) {
	mock := &ProductsMock{
		CreateProductFunc: func(ctx context.Context, product models.Product) (int, error) {
			return 0, errors.New("DB error")
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/products",
		strings.NewReader(`{"Name":"Cat Toy","Price":9.99,"Stock":5}`))
	w := httptest.NewRecorder()

	New(slog.Default(), mock).CreateProduct(w, req)

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
	var got models.Product
	mock := &ProductsMock{
		UpdateProductFunc: func(ctx context.Context, product models.Product) error {
			got = product
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPut, "/products/7",
		strings.NewReader(`{"Name":"Bird Cage","Price":50,"Stock":2}`))
	req = withURLParam(req, "id", "7")
	w := httptest.NewRecorder()

	New(slog.Default(), mock).UpdateProduct(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// ID должен прийти из URL, а не из тела
	if got.ID != 7 || got.Name != "Bird Cage" {
		t.Fatalf("storage got unexpected product: %+v", got)
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
	mock := &ProductsMock{
		UpdateProductFunc: func(ctx context.Context, product models.Product) error {
			t.Fatal("storage must not be called on invalid JSON")
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPut, "/products/7", strings.NewReader(`{bad json`))
	req = withURLParam(req, "id", "7")
	w := httptest.NewRecorder()

	New(slog.Default(), mock).UpdateProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Invalid JSON payload" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestUpdateProduct_InvalidID(t *testing.T) {
	mock := &ProductsMock{
		UpdateProductFunc: func(ctx context.Context, product models.Product) error {
			t.Fatal("storage must not be called with non-numeric id")
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPut, "/products/abc",
		strings.NewReader(`{"Name":"Bird Cage","Price":50,"Stock":2}`))
	req = withURLParam(req, "id", "abc")
	w := httptest.NewRecorder()

	New(slog.Default(), mock).UpdateProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Product ID must be a number" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestUpdateProduct_Fail(t *testing.T) {
	mock := &ProductsMock{
		UpdateProductFunc: func(ctx context.Context, product models.Product) error {
			// Ошибка без "not found", иначе хендлер ответит 404
			return errors.New("DB error")
		},
	}

	req := httptest.NewRequest(http.MethodPut, "/products/7",
		strings.NewReader(`{"Name":"Bird Cage","Price":50,"Stock":2}`))
	req = withURLParam(req, "id", "7")
	w := httptest.NewRecorder()

	New(slog.Default(), mock).UpdateProduct(w, req)

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
	var gotID int
	mock := &ProductsMock{
		DeleteProductFunc: func(ctx context.Context, id int) error {
			gotID = id
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/products/3", nil)
	req = withURLParam(req, "id", "3")
	w := httptest.NewRecorder()

	New(slog.Default(), mock).DeleteProduct(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotID != 3 {
		t.Fatalf("expected storage to delete id 3, got %d", gotID)
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
	mock := &ProductsMock{
		DeleteProductFunc: func(ctx context.Context, id int) error {
			t.Fatal("storage must not be called with empty id")
			return nil
		},
	}

	// id не кладём в контекст — chi.URLParam вернёт пустую строку
	req := httptest.NewRequest(http.MethodDelete, "/products/", nil)
	w := httptest.NewRecorder()

	New(slog.Default(), mock).DeleteProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Product ID is required" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestDeleteProduct_InvalidID(t *testing.T) {
	mock := &ProductsMock{
		DeleteProductFunc: func(ctx context.Context, id int) error {
			t.Fatal("storage must not be called with non-numeric id")
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/products/abc", nil)
	req = withURLParam(req, "id", "abc")
	w := httptest.NewRecorder()

	New(slog.Default(), mock).DeleteProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Product ID must be a number" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestDeleteProduct_Fail(t *testing.T) {
	mock := &ProductsMock{
		DeleteProductFunc: func(ctx context.Context, id int) error {
			return errors.New("DB error")
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/products/3", nil)
	req = withURLParam(req, "id", "3")
	w := httptest.NewRecorder()

	New(slog.Default(), mock).DeleteProduct(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "Failed to delete product" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}
