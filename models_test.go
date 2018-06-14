package jsonapisdk

import (
	"time"
)

type ModelI18n struct {
	ID   int    `jsonapi:"primary,i18n"`
	Lang string `jsonapi:"attr,language,langtag"`
}

type Model struct {
	ID   int    `jsonapi:"primary,models"`
	Name string `jsonapi:"attr,name"`
}

type Author struct {
	ID    int     `jsonapi:"primary,authors"`
	Name  string  `jsonapi:"attr,name"`
	Blogs []*Blog `jsonapi:"relation,blogs"`
}

type Blog struct {
	ID          int    `jsonapi:"primary,blogs"`
	Lang        string `jsonapi:"attr,language,langtag"`
	CurrentPost *Post  `jsonapi:"relation,current_post"`
}

type Post struct {
	ID        int        `jsonapi:"primary,posts"`
	Title     string     `jsonapi:"attr,title"`
	CreatedAt time.Time  `jsonapi:"attr,created_at"`
	Comments  []*Comment `jsonapi:"relation,comments"`
}

type Comment struct {
	ID   int    `jsonapi:"primary,comments"`
	Body string `jsonapi:"attr,body"`
	Post *Post  `jsonapi:"relation,post,hidden"`
}
