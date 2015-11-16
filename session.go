type manager struct {
	cookieName string
	sync.Mutex
	provider provider
	maxlifetime int64
}

type provider interface {
	SessionInit(sid string) (session, error)
	SessionRead(sid string) (session, error)
	SessionDestroy(sid string) (session, error)
	SessionGC(maxLifeTime int64)
}

type session interface {
	Set(key, value interface{}) error
	Get(key, value interface{}) interface{}
	Delete(key interface{}) error
	SessionID() string
}

var globalSessions *manager
var provides = make(map[string]provider)

func Register(name string, provider provider) {
	if provider == nil {
		panic("session: Register provide is nil")
	}
	if _, dup := provides[name]; dup {
		panic("session: Register called twice for provide " + name)
	}
	provides[name] = provider
}

func NewManager(provideName, cookieName string, maxlifetime int64) (*manager, error) {
	provider, ok := provides[provideName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provide %s (forgotten import?)", provideName)
	}
	return &manager{provider: provider, cookieName: cookieName, maxlifetime: maxlifetime}, nil
}

func (manager *manager) sessionID() string {
	b := make([] byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

func (manager *manager) SessionStart(w http.ResponseWriter, r *http.Request) (session session) {
	manager.Lock()
	defer manager.Unlock()
	cookie, err := r.Cookie(manager.cookieName)
	if err != nil || cookie.Value == "" {
		sid := manager.sessionID()
		session, _ = manager.provider.SessionInit(sid)
		cookie := http.Cookie{Name: manager.cookieName, Value: url.QueryEscape(sid),
				Path:"/", HttpOnly: true, MaxAge: int(manager.maxlifetime)}
		http.SetCookie(w, &cookie)
	} else {
		sid, _ := url.QueryUnescape(cookie.Value)
		session, _ = manager.provider.SessionRead(sid)
	}
	return
}

func (manager *manager) SessionDestroy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(manager.cookieName)
	if err != nil || cookie.value == "" {
		return
	} else {
		manager.Lock()
		defer manager.Unlock
		manager.provider.SessionDestroy(cookie.Value)
		expiration := time.Now()
		cookie := http.Cookie{Name : manager.cookieName, Path: "/",
				HttpOnly: true, Expires: expiration, Maxage: -1}
		http.SetCookie(w, &cookie)
	}
}

func (manager *manager) GC() {
	manager.Lock()
	defer manager.Unlock()
	manager.provider.SessionGC(manager.maxlifetime)
	time.AfterFunc(time.Duration(manager.maxlifetime), func() { manager.GC()})
}
