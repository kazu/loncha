
- [X] loncha.Select/FindAll
    条件にあうものを探す(非破壊的)
- [X] loncha.Filter 
    条件にあうものを探す(破壊)
- [x] loncha.Find 
    条件にあう最初ものを返す
- [x] loncha.Contains　
    含まれているか?
- [x] loncha.Delete 
    条件にあったデータを消す
- [X] loncha.Uniq 重複削除(存在確認にinterface cost がでかい)
- [x] loncha.IndexOf index を取る
- [x] loncha.Shuffle 
- [ ] loncha.Step()  []int を作る。
- [ ] loncha.Conv   
    slice からslice を変換
- [ ] loncha.Parallel 
- [X] sql like function(gen を使う)
## loncha.countaer_list

## loncha.list_encabezado

- [x] concurrent 対応
  - [x] add のlock free 実装
  - [x] delete のlock free 実装
  - [x] add の二回目のcas が失敗したときのrollback の調査
- [x] concurrent map 追加
  - [ ] read map cache
  - [ ] support interface key
  - [ ] sharding by key  

## loncha.ecache
