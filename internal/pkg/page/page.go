package page

const (
	defaultPageSize = 10
	defaultPageNum  = 1
	maxPageSize     = 100
)

type PageRequest struct {
	PageSize   int  `json:"pageSize"`
	PageNum    int  `json:"pageNum"`
	normalized bool `json:"-"`
}

func (p *PageRequest) normalize() {
	if p.normalized {
		return
	}
	if p.PageSize <= 0 {
		p.PageSize = defaultPageSize
	}
	if p.PageSize > maxPageSize {
		p.PageSize = maxPageSize
	}
	if p.PageNum <= 0 {
		p.PageNum = defaultPageNum
	}
	p.normalized = true
}

func (p *PageRequest) Offset() int {
	p.normalize()
	return (p.PageNum - 1) * p.PageSize
}

func (p *PageRequest) Limit() int {
	p.normalize()
	return p.PageSize
}

type PageResponse[T any] struct {
	Records  []T   `json:"records"`
	Total    int64 `json:"total"`
	PageSize int   `json:"pageSize"`
	PageNum  int   `json:"pageNum"`
}

func NewPageResponse[T any](records []T, total int64, req PageRequest) *PageResponse[T] {
	return &PageResponse[T]{
		Records:  records,
		Total:    total,
		PageSize: req.PageSize,
		PageNum:  req.PageNum,
	}
}
