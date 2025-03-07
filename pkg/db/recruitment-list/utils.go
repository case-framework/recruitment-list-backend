package recruitmentlist

const (
	FALLBACK_PAGE_SIZE = 10
)

func prepPaginationInfos(totalCount int64, page int64, limit int64) PaginationInfos {
	if limit == 0 {
		limit = FALLBACK_PAGE_SIZE
	}

	if totalCount < limit {
		page = 1
	}

	if page < 1 {
		page = 1
	}

	return PaginationInfos{
		PageSize:    limit,
		TotalCount:  totalCount,
		TotalPages:  getTotalPages(totalCount, limit),
		CurrentPage: page,
	}
}

func getTotalPages(totalCount int64, limit int64) int64 {
	if limit == 0 {
		return 0
	}
	return (totalCount + limit - 1) / limit
}
