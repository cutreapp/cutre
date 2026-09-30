package usecase

import (
	"context"
	"fmt"
	"slices"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// matchesResult は findMatches の結果。
type matchesResult struct {
	// Matches は相手ごとの、ユーザーとの間で交換できるアイテム。交換できるものが無い相手も空の Match で入れる。
	Matches map[model.UserID]model.Match
	// Goods・EventCategories は、交換できるアイテムのグッズと、そのカテゴリー。名前の表示に使う。マスタの状態は問わない。
	Goods           map[model.GoodsID]*model.Goods
	EventCategories map[model.EventCategoryID]*model.EventCategory
}

// findMatches は、ユーザー userID と相手 partnerIDs それぞれの間で交換できるアイテムを、グッズとカテゴリーと一緒に返す。
//
// アイテムは「相手のもの」と「ユーザーのもの」の2回のクエリで、マスタは段ごとに1回のクエリでまとめて引き、
// 相手の数によらずクエリの回数を一定にする。
func findMatches(
	ctx context.Context,
	itemRepo *repository.ItemRepository,
	goodsRepo *repository.GoodsRepository,
	eventCategoryRepo *repository.EventCategoryRepository,
	userID model.UserID,
	partnerIDs []model.UserID,
) (*matchesResult, error) {
	partnerItems, err := itemRepo.ListListedMatching(ctx, partnerIDs, []model.UserID{userID})
	if err != nil {
		return nil, fmt.Errorf("相手の交換できるアイテムの取得に失敗: %w", err)
	}
	userItems, err := itemRepo.ListListedMatching(ctx, []model.UserID{userID}, partnerIDs)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの交換できるアイテムの取得に失敗: %w", err)
	}

	itemsByPartner := make(map[model.UserID][]*model.Item, len(partnerIDs))
	for _, item := range partnerItems {
		itemsByPartner[item.UserID] = append(itemsByPartner[item.UserID], item)
	}
	matches := make(map[model.UserID]model.Match, len(partnerIDs))
	var goodsIDs []model.GoodsID
	for _, partnerID := range partnerIDs {
		// 相手のアイテムを先に並べ、もらえるものも渡せるものもリストと同じ並びにする。
		match := model.NewMatch(userID, partnerID, slices.Concat(itemsByPartner[partnerID], userItems))
		matches[partnerID] = match
		for _, matchItem := range slices.Concat(match.Receivable, match.Givable) {
			goodsIDs = append(goodsIDs, matchItem.Item.GoodsID)
		}
	}

	goods, categories, err := findGoodsWithCategories(ctx, goodsRepo, eventCategoryRepo, goodsIDs)
	if err != nil {
		return nil, err
	}

	return &matchesResult{Matches: matches, Goods: goods, EventCategories: categories}, nil
}

// findGoodsWithCategories は、グッズ goodsIDs と、そのカテゴリーを、それぞれ1回のクエリでまとめて引き、IDで引けるmapにして返す。
// アイテムの名前の表示に使うため、マスタの状態は問わない。
func findGoodsWithCategories(
	ctx context.Context,
	goodsRepo *repository.GoodsRepository,
	eventCategoryRepo *repository.EventCategoryRepository,
	goodsIDs []model.GoodsID,
) (map[model.GoodsID]*model.Goods, map[model.EventCategoryID]*model.EventCategory, error) {
	goodsList, err := goodsRepo.ListByIDs(ctx, goodsIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("グッズの取得に失敗: %w", err)
	}
	goods := make(map[model.GoodsID]*model.Goods, len(goodsList))
	categoryIDs := make([]model.EventCategoryID, 0, len(goodsList))
	for _, g := range goodsList {
		goods[g.ID] = g
		categoryIDs = append(categoryIDs, g.EventCategoryID)
	}

	categoryList, err := eventCategoryRepo.ListByIDs(ctx, categoryIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("カテゴリーの取得に失敗: %w", err)
	}
	categories := make(map[model.EventCategoryID]*model.EventCategory, len(categoryList))
	for _, category := range categoryList {
		categories[category.ID] = category
	}

	return goods, categories, nil
}
