package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/inventory"
	"gmc/api/internal/platform"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ListCategories implements GET /v1/categories
func (s *Server) ListCategories(w http.ResponseWriter, r *http.Request) {
	val, err := s.cache.DoSingleflight("catalog:categories", 5*time.Minute, func() (any, error) {
		rows, err := s.q.ListPublicCategories(r.Context())
		if err != nil {
			return nil, err
		}
		items := make([]openapi.Category, 0, len(rows))
		for _, row := range rows {
			cnt := int(row.ProductCount)
			so := int(row.SortOrder)
			items = append(items, openapi.Category{
				Id:           row.ID,
				Slug:         row.Slug,
				Name:         row.Name,
				ParentSlug:   row.ParentSlug,
				Description:  row.Description,
				ImageUrl:     row.ImageUrl,
				SortOrder:    &so,
				ProductCount: &cnt,
			})
		}
		return map[string]any{"items": items}, nil
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteCatalogJSON(w, r, val)
}

// ListBrands implements GET /v1/brands
func (s *Server) ListBrands(w http.ResponseWriter, r *http.Request) {
	val, err := s.cache.DoSingleflight("catalog:brands", 5*time.Minute, func() (any, error) {
		rows, err := s.q.ListPublicBrands(r.Context())
		if err != nil {
			return nil, err
		}
		items := make([]openapi.Brand, 0, len(rows))
		for _, row := range rows {
			cnt := int(row.ProductCount)
			items = append(items, openapi.Brand{
				Id:           row.ID,
				Slug:         row.Slug,
				Name:         row.Name,
				ProductCount: &cnt,
			})
		}
		return map[string]any{"items": items}, nil
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteCatalogJSON(w, r, val)
}

type productFilter struct {
	q          string
	categories []string
	brands     []string
	minPrice   *int64
	maxPrice   *int64
	inStock    bool
	featured   *bool
	sortBy     string
	page       int
	pageSize   int
}

// ListProducts implements GET /v1/products
func (s *Server) ListProducts(w http.ResponseWriter, r *http.Request, params openapi.ListProductsParams) {
	page := 1
	if params.Page != nil && *params.Page >= 1 {
		page = *params.Page
	}
	pageSize := 24
	if params.PageSize != nil && *params.PageSize >= 1 && *params.PageSize <= 60 {
		pageSize = *params.PageSize
	}

	var cats, brands []string
	if params.Category != nil {
		for _, c := range *params.Category {
			for _, part := range strings.Split(c, ",") {
				if p := strings.TrimSpace(part); p != "" {
					cats = append(cats, p)
				}
			}
		}
	}
	if params.Brand != nil {
		for _, b := range *params.Brand {
			for _, part := range strings.Split(b, ",") {
				if p := strings.TrimSpace(part); p != "" {
					brands = append(brands, p)
				}
			}
		}
	}
	sort.Strings(cats)
	sort.Strings(brands)

	q := ""
	if params.Q != nil {
		q = strings.TrimSpace(*params.Q)
	}
	sortBy := "newest"
	if params.Sort != nil {
		sortBy = string(*params.Sort)
	}
	inStock := params.InStock != nil && *params.InStock

	pf := productFilter{
		q:          q,
		categories: cats,
		brands:     brands,
		minPrice:   params.MinPriceIdr,
		maxPrice:   params.MaxPriceIdr,
		inStock:    inStock,
		featured:   params.Featured,
		sortBy:     sortBy,
		page:       page,
		pageSize:   pageSize,
	}

	cacheKey := fmt.Sprintf("catalog:list:q=%s|c=%s|b=%s|min=%v|max=%v|s=%v|f=%v|sort=%s|p=%d|ps=%d",
		pf.q, strings.Join(pf.categories, ","), strings.Join(pf.brands, ","),
		int64PtrStr(pf.minPrice), int64PtrStr(pf.maxPrice), pf.inStock, boolPtrStr(pf.featured),
		pf.sortBy, pf.page, pf.pageSize)

	val, err := s.cache.DoSingleflight(cacheKey, 60*time.Second, func() (any, error) {
		return s.queryPublicProducts(r.Context(), pf)
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteCatalogJSON(w, r, val)
}

func int64PtrStr(p *int64) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", *p)
}

func boolPtrStr(p *bool) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%v", *p)
}

func buildProductWhere(pf productFilter, includeCat, includeBrand, includePrice bool) (string, []any) {
	clauses := []string{
		"p.is_active = true",
		"p.deleted_at IS NULL",
		"b.is_active = true",
		"c.is_active = true",
		"vstats.variant_count > 0",
	}
	var args []any
	addArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if pf.q != "" {
		ph := addArg(pf.q)
		clauses = append(clauses, fmt.Sprintf("(p.search_vector @@ websearch_to_tsquery('simple', unaccent(%s)) OR similarity(p.name, %s) > 0.2 OR p.name ILIKE '%%' || %s || '%%')", ph, ph, ph))
	}
	if includeCat && len(pf.categories) > 0 {
		ph := addArg(pf.categories)
		clauses = append(clauses, fmt.Sprintf("c.slug = ANY(%s)", ph))
	}
	if includeBrand && len(pf.brands) > 0 {
		ph := addArg(pf.brands)
		clauses = append(clauses, fmt.Sprintf("b.slug = ANY(%s)", ph))
	}
	if includePrice && pf.minPrice != nil {
		ph := addArg(*pf.minPrice)
		clauses = append(clauses, fmt.Sprintf("vstats.max_price >= %s", ph))
	}
	if includePrice && pf.maxPrice != nil {
		ph := addArg(*pf.maxPrice)
		clauses = append(clauses, fmt.Sprintf("vstats.min_price <= %s", ph))
	}
	if pf.inStock {
		clauses = append(clauses, "(vstats.total_stock > 0 OR vstats.has_preorder = true)")
	}
	if pf.featured != nil {
		ph := addArg(*pf.featured)
		clauses = append(clauses, fmt.Sprintf("p.is_featured = %s", ph))
	}

	return strings.Join(clauses, " AND "), args
}

const variantStatsLateralSQL = `
LEFT JOIN LATERAL (
  SELECT
    count(*)::int AS variant_count,
    COALESCE(min(v.price_idr), 0)::bigint AS min_price,
    COALESCE(max(v.price_idr), 0)::bigint AS max_price,
    min(v.compare_at_price_idr)::bigint AS min_compare_at,
    COALESCE(sum(v.stock), 0)::int AS total_stock,
    COALESCE(min(v.low_stock_threshold), 3)::int AS min_threshold,
    bool_or(v.is_preorder) AS has_preorder,
    (array_agg(v.id ORDER BY (v.stock > 0) DESC, v.price_idr ASC))[1] AS default_variant_id
  FROM variants v
  WHERE v.product_id = p.id AND v.is_active = true
) vstats ON true
`

func (s *Server) queryPublicProducts(ctx context.Context, pf productFilter) (*openapi.ProductListResponse, error) {
	whereSQL, args := buildProductWhere(pf, true, true, true)

	// 1. Count total
	countQuery := fmt.Sprintf(`
		SELECT count(*)::int
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		JOIN categories c ON c.id = p.category_id
		%s
		WHERE %s
	`, variantStatsLateralSQL, whereSQL)

	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	orderBy := "p.created_at DESC"
	switch pf.sortBy {
	case "price_asc":
		orderBy = "vstats.min_price ASC, p.created_at DESC"
	case "price_desc":
		orderBy = "vstats.min_price DESC, p.created_at DESC"
	case "name_asc":
		orderBy = "p.name ASC"
	default:
		if pf.q != "" {
			orderBy = "similarity(p.name, $1) DESC, p.created_at DESC"
		}
	}

	offset := (pf.page - 1) * pf.pageSize
	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, pf.pageSize, offset)
	limitPH := fmt.Sprintf("$%d", len(listArgs)-1)
	offsetPH := fmt.Sprintf("$%d", len(listArgs))

	listQuery := fmt.Sprintf(`
		SELECT
			p.id, p.slug, p.name, p.summary, p.is_featured,
			b.id, b.slug, b.name,
			c.id, c.slug, c.name,
			vstats.variant_count, vstats.min_price, vstats.max_price, vstats.min_compare_at,
			vstats.total_stock, vstats.min_threshold, COALESCE(vstats.has_preorder, false),
			vstats.default_variant_id,
			COALESCE(imgs.images_json, '[]'::jsonb)
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		JOIN categories c ON c.id = p.category_id
		%s
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object(
				'url', pi.url,
				'alt', pi.alt,
				'width', pi.width,
				'height', pi.height,
				'kind', pi.kind
			) ORDER BY pi.sort_order ASC) AS images_json
			FROM (
				SELECT * FROM product_images pi2
				WHERE pi2.product_id = p.id
				ORDER BY pi2.sort_order ASC
				LIMIT 2
			) pi
		) imgs ON true
		WHERE %s
		ORDER BY %s
		LIMIT %s OFFSET %s
	`, variantStatsLateralSQL, whereSQL, orderBy, limitPH, offsetPH)

	rows, err := s.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]openapi.ProductCard, 0)
	for rows.Next() {
		var card openapi.ProductCard
		var brand openapi.Brand
		var cat openapi.Category
		var vCount, totalStock, minThreshold int32
		var hasPreorder bool
		var defaultVarID uuid.UUID
		var imgsRaw []byte

		if err := rows.Scan(
			&card.Id, &card.Slug, &card.Name, &card.Summary, &card.IsFeatured,
			&brand.Id, &brand.Slug, &brand.Name,
			&cat.Id, &cat.Slug, &cat.Name,
			&vCount, &card.PriceFromIdr, &card.PriceToIdr, &card.CompareAtPriceFromIdr,
			&totalStock, &minThreshold, &hasPreorder,
			&defaultVarID,
			&imgsRaw,
		); err != nil {
			return nil, err
		}
		card.Brand = brand
		card.Category = cat
		card.HasVariants = vCount > 1
		card.DefaultVariantId = &defaultVarID
		card.StockStatus = inventory.ComputeStockStatus(totalStock, minThreshold, hasPreorder)
		_ = json.Unmarshal(imgsRaw, &card.Images)
		if card.Images == nil {
			card.Images = []openapi.Image{}
		}
		items = append(items, card)
	}

	// Compute facets (category facet excludes category filter, brand facet excludes brand filter, priceRange excludes price filter)
	facets, err := s.computeProductFacets(ctx, pf)
	if err != nil {
		return nil, err
	}

	totalPages := (total + pf.pageSize - 1) / pf.pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	return &openapi.ProductListResponse{
		Items:      items,
		Page:       pf.page,
		PageSize:   pf.pageSize,
		Total:      total,
		TotalPages: totalPages,
		Facets:     *facets,
	}, nil
}

func (s *Server) computeProductFacets(ctx context.Context, pf productFilter) (*openapi.ProductFacets, error) {
	facets := &openapi.ProductFacets{
		Categories: []openapi.FacetBucket{},
		Brands:     []openapi.FacetBucket{},
	}

	// Category facets (exclude category filter)
	catWhere, catArgs := buildProductWhere(pf, false, true, true)
	catRows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT c.slug, count(*)::int
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		JOIN categories c ON c.id = p.category_id
		%s
		WHERE %s
		GROUP BY c.slug, c.sort_order
		ORDER BY c.sort_order ASC
	`, variantStatsLateralSQL, catWhere), catArgs...)
	if err == nil {
		for catRows.Next() {
			var b openapi.FacetBucket
			if catRows.Scan(&b.Slug, &b.Count) == nil {
				facets.Categories = append(facets.Categories, b)
			}
		}
		catRows.Close()
	}

	// Brand facets (exclude brand filter)
	brandWhere, brandArgs := buildProductWhere(pf, true, false, true)
	brandRows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT b.slug, count(*)::int
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		JOIN categories c ON c.id = p.category_id
		%s
		WHERE %s
		GROUP BY b.slug, b.name
		ORDER BY b.name ASC
	`, variantStatsLateralSQL, brandWhere), brandArgs...)
	if err == nil {
		for brandRows.Next() {
			var b openapi.FacetBucket
			if brandRows.Scan(&b.Slug, &b.Count) == nil {
				facets.Brands = append(facets.Brands, b)
			}
		}
		brandRows.Close()
	}

	// Price range (exclude price filter)
	priceWhere, priceArgs := buildProductWhere(pf, true, true, false)
	var minP, maxP int64
	_ = s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(min(vstats.min_price), 0)::bigint, COALESCE(max(vstats.max_price), 0)::bigint
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		JOIN categories c ON c.id = p.category_id
		%s
		WHERE %s
	`, variantStatsLateralSQL, priceWhere), priceArgs...).Scan(&minP, &maxP)
	facets.PriceRange.MinIdr = minP
	facets.PriceRange.MaxIdr = maxP

	return facets, nil
}

// GetProductBySlug implements GET /v1/products/{slug}
func (s *Server) GetProductBySlug(w http.ResponseWriter, r *http.Request, slug string) {
	val, err := s.cache.DoSingleflight("catalog:detail:"+slug, 60*time.Second, func() (any, error) {
		return s.loadProductDetailBySlug(r.Context(), slug)
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteCatalogJSON(w, r, val)
}

// LoadProductDetailWithQueryCount loads product detail in <= 5 queries (used by both handler and query-count test).
func (s *Server) loadProductDetailBySlug(ctx context.Context, slug string) (*openapi.ProductDetail, error) {
	// Query 1: Product + Brand + Category
	row, err := s.q.GetProductBySlug(ctx, slug)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Produk tidak ditemukan")
		}
		return nil, err
	}

	// Query 2: Images
	imgRows, err := s.q.ListProductImages(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	images := make([]openapi.Image, 0, len(imgRows))
	for _, im := range imgRows {
		var wPtr, hPtr *int
		if im.Width != nil {
			v := int(*im.Width)
			wPtr = &v
		}
		if im.Height != nil {
			v := int(*im.Height)
			hPtr = &v
		}
		images = append(images, openapi.Image{
			Url:    im.Url,
			Alt:    im.Alt,
			Width:  wPtr,
			Height: hPtr,
			Kind:   openapi.ImageKind(im.Kind),
		})
	}

	// Query 3: Options
	optRows, err := s.q.ListProductOptions(ctx, row.ID)
	if err != nil {
		return nil, err
	}

	// Query 4: Option Values
	valRows, err := s.q.ListProductOptionValuesByProduct(ctx, row.ID)
	if err != nil {
		return nil, err
	}

	valByOption := make(map[uuid.UUID][]openapi.ProductOptionValue)
	for _, vr := range valRows {
		valByOption[vr.OptionID] = append(valByOption[vr.OptionID], openapi.ProductOptionValue{
			Id:        vr.ID,
			Label:     vr.Label,
			SwatchHex: vr.SwatchHex,
		})
	}

	options := make([]openapi.ProductOption, 0, len(optRows))
	for _, op := range optRows {
		vals := valByOption[op.ID]
		if vals == nil {
			vals = []openapi.ProductOptionValue{}
		}
		options = append(options, openapi.ProductOption{
			Id:      op.ID,
			Name:    op.Name,
			Display: openapi.OptionDisplay(op.Display),
			Values:  vals,
		})
	}

	// Query 5: Variants
	varRows, err := s.q.ListActiveVariantsByProduct(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	if len(varRows) == 0 {
		return nil, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Produk belum memiliki varian aktif")
	}

	variants := make([]openapi.ProductVariant, 0, len(varRows))
	var minPrice, maxPrice int64
	var minCompareAt *int64
	var totalStock int32
	var minThreshold int32 = 3
	var hasPreorder bool

	for i, vr := range varRows {
		if i == 0 || vr.PriceIdr < minPrice {
			minPrice = vr.PriceIdr
		}
		if i == 0 || vr.PriceIdr > maxPrice {
			maxPrice = vr.PriceIdr
		}
		if vr.CompareAtPriceIdr != nil && (minCompareAt == nil || *vr.CompareAtPriceIdr < *minCompareAt) {
			v := *vr.CompareAtPriceIdr
			minCompareAt = &v
		}
		totalStock += vr.Stock
		if vr.IsPreorder {
			hasPreorder = true
		}
		ovIDs := vr.OptionValueIds
		if ovIDs == nil {
			ovIDs = []uuid.UUID{}
		}
		variants = append(variants, openapi.ProductVariant{
			Id:               vr.ID,
			Sku:              vr.Sku,
			OptionValueIds:   ovIDs,
			PriceIdr:         vr.PriceIdr,
			CompareAtPriceIdr: vr.CompareAtPriceIdr,
			Stock:            int(vr.Stock),
			StockStatus:      inventory.ComputeStockStatus(vr.Stock, vr.LowStockThreshold, vr.IsPreorder),
			ImageUrl:         vr.ImageUrl,
		})
	}

	specs := make(map[string]string)
	_ = json.Unmarshal(row.Specs, &specs)

	highlights := row.Highlights
	if highlights == nil {
		highlights = []string{}
	}

	productURL := fmt.Sprintf("%s/shop/%s", s.cfg.SiteURL, row.Slug)
	waURL := platform.BuildProductWhatsAppURL(s.cfg.WhatsAppNumber, row.Name, productURL)

	return &openapi.ProductDetail{
		Id:                    row.ID,
		Slug:                  row.Slug,
		Name:                  row.Name,
		Summary:               row.Summary,
		Description:           row.Description,
		Highlights:            highlights,
		Specs:                 specs,
		Brand:                 openapi.Brand{Id: row.BrandID, Slug: row.BrandSlug, Name: row.BrandName},
		Category:              openapi.Category{Id: row.CategoryID, Slug: row.CategorySlug, Name: row.CategoryName},
		PriceFromIdr:          minPrice,
		PriceToIdr:            maxPrice,
		CompareAtPriceFromIdr: minCompareAt,
		Images:                images,
		StockStatus:           inventory.ComputeStockStatus(totalStock, minThreshold, hasPreorder),
		IsFeatured:            row.IsFeatured,
		HasVariants:           len(variants) > 1,
		Options:               options,
		Variants:              variants,
		WhatsappUrl:           waURL,
		SeoTitle:              row.SeoTitle,
		SeoDescription:        row.SeoDescription,
	}, nil
}

// GetRelatedProducts implements GET /v1/products/{slug}/related
func (s *Server) GetRelatedProducts(w http.ResponseWriter, r *http.Request, slug string) {
	val, err := s.cache.DoSingleflight("catalog:related:"+slug, 60*time.Second, func() (any, error) {
		p, err := s.q.GetProductBySlug(r.Context(), slug)
		if err != nil {
			return map[string]any{"items": []openapi.ProductCard{}}, nil
		}
		// Curated related first, fallback to same category
		pf := productFilter{
			categories: []string{p.CategorySlug},
			page:       1,
			pageSize:   9,
		}
		res, err := s.queryPublicProducts(r.Context(), pf)
		if err != nil {
			return nil, err
		}
		var filtered []openapi.ProductCard
		for _, item := range res.Items {
			if item.Slug != slug && len(filtered) < 8 {
				filtered = append(filtered, item)
			}
		}
		if filtered == nil {
			filtered = []openapi.ProductCard{}
		}
		return map[string]any{"items": filtered}, nil
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteCatalogJSON(w, r, val)
}

// ResolveVariant implements GET /v1/variants/resolve
func (s *Server) ResolveVariant(w http.ResponseWriter, r *http.Request, params openapi.ResolveVariantParams) {
	if params.Sku != nil && strings.TrimSpace(*params.Sku) != "" {
		vr, err := s.q.GetVariantBySKU(r.Context(), strings.TrimSpace(*params.Sku))
		if err != nil || !vr.IsActive || !vr.ProductActive || vr.ProductDeletedAt != nil {
			platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Varian tidak ditemukan"))
			return
		}
		ovIDs := vr.OptionValueIds
		if ovIDs == nil {
			ovIDs = []uuid.UUID{}
		}
		platform.WriteCatalogJSON(w, r, openapi.ResolvedVariantResponse{
			ProductSlug: vr.ProductSlug,
			Variant: openapi.ProductVariant{
				Id:                vr.ID,
				Sku:               vr.Sku,
				OptionValueIds:    ovIDs,
				PriceIdr:          vr.PriceIdr,
				CompareAtPriceIdr: vr.CompareAtPriceIdr,
				Stock:             int(vr.Stock),
				StockStatus:       inventory.ComputeStockStatus(vr.Stock, vr.LowStockThreshold, vr.IsPreorder),
				ImageUrl:          vr.ImageUrl,
			},
		})
		return
	}

	if params.ProductSlug != nil && strings.TrimSpace(*params.ProductSlug) != "" {
		detail, err := s.loadProductDetailBySlug(r.Context(), strings.TrimSpace(*params.ProductSlug))
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		if len(detail.Variants) == 0 {
			platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Varian tidak ditemukan"))
			return
		}
		if params.OptionValueIds != nil && strings.TrimSpace(*params.OptionValueIds) != "" {
			wanted := make(map[string]bool)
			for _, part := range strings.Split(*params.OptionValueIds, ",") {
				if p := strings.TrimSpace(part); p != "" {
					wanted[p] = true
				}
			}
			for _, v := range detail.Variants {
				if len(v.OptionValueIds) == len(wanted) {
					allMatch := true
					for _, id := range v.OptionValueIds {
						if !wanted[id.String()] {
							allMatch = false
							break
						}
					}
					if allMatch {
						platform.WriteCatalogJSON(w, r, openapi.ResolvedVariantResponse{
							ProductSlug: detail.Slug,
							Variant:     v,
						})
						return
					}
				}
			}
		}
		platform.WriteCatalogJSON(w, r, openapi.ResolvedVariantResponse{
			ProductSlug: detail.Slug,
			Variant:     detail.Variants[0],
		})
		return
	}

	platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.BadRequest, "Parameter sku atau productSlug wajib diisi"))
}
