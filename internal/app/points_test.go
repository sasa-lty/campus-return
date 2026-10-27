package app

import (
 "campusreturn/internal/domain"
 "database/sql"
 "strings"
 "testing"
)
func TestContributionLimitsAndLocalDay(t *testing.T){s,e:=OpenStore(t.TempDir(),strings.Repeat("e",64));if e!=nil{t.Fatal(e)};defer s.DB.Close();srv:=&Server{S:s};award:=func(id,owner,at string){t.Helper();if e=s.Transaction(func(q *sql.Tx)error{return srv.award(q,domain.Item{ID:id,ReporterID:"a",OwnerID:owner},at)});e!=nil{t.Fatal(e)}};score:=func()int{v,e:=records[Point](s.DB,"point",`AND json_extract(data,'$.userId')='a'`);if e!=nil{t.Fatal(e)};n:=0;for _,p:=range v{n+=p.Delta};return n};award("one","b","2026-10-23T15:00:00Z");award("same-pair","b","2026-10-23T15:01:00Z");if score()!=10{t.Fatal("same pair scored twice")};award("two","c","2026-10-23T15:02:00Z");award("capped","d","2026-10-23T15:03:00Z");if score()!=20{t.Fatal("daily cap failed",score())};award("next-day","e","2026-10-23T16:01:00Z");if score()!=30{t.Fatal("Beijing midnight did not reset cap",score())};award("next-week","b","2026-11-01T15:00:00Z");if score()!=40{t.Fatal("pair restriction did not expire")}}
