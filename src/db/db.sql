-- ALTER TABLE transaction
--     DRop COLUMN handled;

ALTER TABLE transaction
    ADD COLUMN handled BOOLEAN DEFAULT false;
CREATE INDEX idx_transaction_handled ON transaction (handled);

alter table transaction
    add constraint tx_hash_unique
        unique (hash);

alter table "tokenTransfer"
    add constraint token_token_unique
        unique ("transactionHash", "logIndex", "tokenId");

alter table transaction
    alter column fee type numeric(256) using fee::numeric(256);

alter table "balanceChange"
    drop constraint "balanceChangeAddressContractUnique";

-- tokenTransfer

create index idx_contract
    on "tokenTransfer" (contract);

CREATE INDEX idx_tokenTransfer_contract_tokenType ON "tokenTransfer"("contract", "tokenType");

CREATE INDEX "idx_tokenTransfer_transactionHash_logIndex"
    ON "tokenTransfer" ("transactionHash", "logIndex");

-- holders

create table if not exists "tokenBalanceChangeHandled"
(
    txhash     varchar(80),
    "logIndex" bigint
);

create index if not exists "tmpTokenTransferHandled_txhash_logIndex_index"
    on "tokenBalanceChangeHandled" (txhash, "logIndex");

