//These are the sql queries used to init flow control -> must be connected to DB port 
//Defaults are set in place for all other columns so only flowName is needed.
INSERT IGNORE INTO TierceronFlow(flowName) VALUES ("{flowName}");

//To set into initialization state
update TierceronFlow set state=1 where flowName='{flowName}'; //To initialize the flow
update TierceronFlow set state=3 where flowName='{flowName}'; //To shutdown the flow

//To set desired syncMode
update TierceronFlow set syncMode="nosync" where flowName='{flowName}';
update TierceronFlow set syncMode="push" where flowName='{flowName}';
update TierceronFlow set syncMode="pull" where flowName='{flowName}';
update TierceronFlow set syncMode="pushonce" where flowName='{flowName}';
update TierceronFlow set syncMode="pullonce" where flowName='{flowName}';

//Trcx command to retrieve row
trcx -env=QA -token=$VAULT_TOKEN -insecure -indexed=FlumeDatabase -serviceFilter=TierceronFlow -indexFilter=flowName

select * from TierceronFlow
-----
STATE
-----
0 stopped -> Flow is stopped
1 start -> This is used to restart the flow ->  wipes table -> sets itself to 2
2 running, -> flow is running 
3 shutdown -> stops running -> sets itself to 0 when finished
4 error -> //When it errors from pulling //Not implemented yet 

--------
SYNCMODE
-------- 
trcdb will always stay synced with vault regardlesss of syncMode

nosync -> trcdb is not pushing or pulling to/from mysql
push -> trcdb is pushing to mysql
pull -> trcdb is pulling from mysql 
pushonce -> trcdb is pulling from mysql once -> sets itself to pushcomplete
pullonce -> trcdb is pulling from mysql once -> sets itself to pullcomplete