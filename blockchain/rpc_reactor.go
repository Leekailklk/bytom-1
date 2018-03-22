package blockchain

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/bytom/blockchain/accesstoken"
	"github.com/bytom/dashboard"
	"github.com/bytom/errors"
	"github.com/bytom/net/http/authn"
	"github.com/bytom/net/http/httpjson"
	"github.com/bytom/net/http/static"
)

var (
	errNotAuthenticated = errors.New("not authenticated")
)

// json Handler
func jsonHandler(f interface{}) http.Handler {
	h, err := httpjson.Handler(f, errorFormatter.Write)
	if err != nil {
		panic(err)
	}
	return h
}

// error Handler
func alwaysError(err error) http.Handler {
	return jsonHandler(func() error { return err })
}

func webAssetsHandler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", static.Handler{
		Assets:  dashboard.Files,
		Default: "index.html",
	}))
	mux.Handle("/", next)

	return mux
}

// BuildHandler is in charge of all the rpc handling.
func (bcr *BlockchainReactor) BuildHandler() {
	m := http.NewServeMux()
	if bcr.wallet != nil && bcr.wallet.AccountMgr != nil && bcr.wallet.AssetReg != nil {
		m.Handle("/create-account", jsonHandler(bcr.createAccount))
		m.Handle("/update-account-tags", jsonHandler(bcr.updateAccountTags))
		m.Handle("/create-account-receiver", jsonHandler(bcr.createAccountReceiver))
		m.Handle("/list-accounts", jsonHandler(bcr.listAccounts))
		m.Handle("/list-addresses", jsonHandler(bcr.listAddresses))
		m.Handle("/delete-account", jsonHandler(bcr.deleteAccount))
		m.Handle("/validate-address", jsonHandler(bcr.validateAddress))

		m.Handle("/create-asset", jsonHandler(bcr.createAsset))
		m.Handle("/update-asset-alias", jsonHandler(bcr.updateAssetAlias))
		m.Handle("/update-asset-tags", jsonHandler(bcr.updateAssetTags))
		m.Handle("/list-assets", jsonHandler(bcr.listAssets))

		m.Handle("/create-key", jsonHandler(bcr.pseudohsmCreateKey))
		m.Handle("/list-keys", jsonHandler(bcr.pseudohsmListKeys))
		m.Handle("/delete-key", jsonHandler(bcr.pseudohsmDeleteKey))
		m.Handle("/reset-key-password", jsonHandler(bcr.pseudohsmResetPassword))

		m.Handle("/get-transaction", jsonHandler(bcr.getTransaction))
		m.Handle("/list-transactions", jsonHandler(bcr.listTransactions))
		m.Handle("/list-balances", jsonHandler(bcr.listBalances))
	} else {
		log.Warn("Please enable wallet")
	}

	m.Handle("/", alwaysError(errors.New("not Found")))

	m.Handle("/build-transaction", jsonHandler(bcr.build))
	m.Handle("/sign-transaction", jsonHandler(bcr.pseudohsmSignTemplates))
	m.Handle("/submit-transaction", jsonHandler(bcr.submit))
	m.Handle("/sign-submit-transaction", jsonHandler(bcr.signSubmit))

	m.Handle("/create-transaction-feed", jsonHandler(bcr.createTxFeed))
	m.Handle("/get-transaction-feed", jsonHandler(bcr.getTxFeed))
	m.Handle("/update-transaction-feed", jsonHandler(bcr.updateTxFeed))
	m.Handle("/delete-transaction-feed", jsonHandler(bcr.deleteTxFeed))
	m.Handle("/list-transaction-feeds", jsonHandler(bcr.listTxFeeds))
	m.Handle("/list-unspent-outputs", jsonHandler(bcr.listUnspentOutputs))
	m.Handle("/info", jsonHandler(bcr.info))

	m.Handle("/create-access-token", jsonHandler(bcr.createAccessToken))
	m.Handle("/list-access-tokens", jsonHandler(bcr.listAccessTokens))
	m.Handle("/delete-access-token", jsonHandler(bcr.deleteAccessToken))
	m.Handle("/check-access-token", jsonHandler(bcr.checkAccessToken))

	m.Handle("/block-hash", jsonHandler(bcr.getBestBlockHash))

	m.Handle("/export-private-key", jsonHandler(bcr.walletExportKey))
	m.Handle("/import-private-key", jsonHandler(bcr.walletImportKey))
	m.Handle("/import-key-progress", jsonHandler(bcr.keyImportProgress))

	m.Handle("/get-block-header-by-hash", jsonHandler(bcr.getBlockHeaderByHash))
	m.Handle("/get-block-header-by-height", jsonHandler(bcr.getBlockHeaderByHeight))
	m.Handle("/get-block", jsonHandler(bcr.getBlock))
	m.Handle("/get-block-count", jsonHandler(bcr.getBlockCount))
	m.Handle("/get-block-transactions-count-by-hash", jsonHandler(bcr.getBlockTransactionsCountByHash))
	m.Handle("/get-block-transactions-count-by-height", jsonHandler(bcr.getBlockTransactionsCountByHeight))

	m.Handle("/net-info", jsonHandler(bcr.getNetInfo))

	m.Handle("/is-mining", jsonHandler(bcr.isMining))
	m.Handle("/gas-rate", jsonHandler(bcr.gasRate))
	m.Handle("/getwork", jsonHandler(bcr.getWork))
	m.Handle("/submitwork", jsonHandler(bcr.submitWork))

	latencyHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if l := latency(m, req); l != nil {
			defer l.RecordSince(time.Now())
		}
		m.ServeHTTP(w, req)
	})
	handler := maxBytes(latencyHandler) // TODO(tessr): consider moving this to non-core specific mux
	handler = webAssetsHandler(handler)

	bcr.Handler = handler
}

//AuthHandler access token auth Handler
func AuthHandler(handler http.Handler, accessTokens *accesstoken.CredentialStore) http.Handler {
	authenticator := authn.NewAPI(accessTokens)

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// TODO(tessr): check that this path exists; return early if this path isn't legit
		req, err := authenticator.Authenticate(req)
		if err != nil {
			log.WithField("error", errors.Wrap(err, "Serve")).Error("Authenticate fail")
			err = errors.Sub(errNotAuthenticated, err)
			errorFormatter.Write(req.Context(), rw, err)
			return
		}
		handler.ServeHTTP(rw, req)
	})
}
