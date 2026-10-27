package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const delaiTest = 100 * time.Millisecond

type banc struct {
	t          *testing.T
	root       string
	cancel     context.CancelFunc
	fini       chan error
	imports    chan int
	n          atomic.Int32
	mu         sync.Mutex
	depotPaths []string
	traiter    func(context.Context, int) (Changements, error)
	annoncer   func(...string)
	logs       chan string
}

func nouveauBanc(t *testing.T, depots ...string) *banc {
	t.Helper()
	b := &banc{t: t, root: t.TempDir(), fini: make(chan error, 1), imports: make(chan int, 128), depotPaths: depots, logs: make(chan string, 128)}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	opt := Options{
		Coffre: b.root, Delai: delaiTest,
		Depots: func() []string { b.mu.Lock(); defer b.mu.Unlock(); return append([]string(nil), b.depotPaths...) },
		Importer: func(ctx context.Context) (Changements, error) {
			n := int(b.n.Add(1))
			b.imports <- n
			b.mu.Lock()
			f := b.traiter
			b.mu.Unlock()
			if f != nil {
				return f(ctx, n)
			}
			return Changements{}, nil
		},
		Annoncer: func(tables ...string) {
			b.mu.Lock()
			f := b.annoncer
			b.mu.Unlock()
			if f != nil {
				f(tables...)
			}
		},
		Journal: func(format string, args ...any) {
			select {
			case b.logs <- format:
			default:
			}
		},
	}
	go func() { b.fini <- Surveiller(ctx, opt) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-b.fini:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("annulation bloquée")
		}
	})
	// Une sonde répétée attend l'installation effective des surveillances.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	pret := false
	for !pret {
		select {
		case <-ticker.C:
			b.ecrire(filepath.Join(b.root, "sonde.md"), "prêt")
		case <-b.imports:
			pret = true
		case <-deadline.C:
			t.Fatal("surveillance non prête")
		}
	}
	b.silence(3 * delaiTest)
	b.n.Store(0)
	return b
}

func (b *banc) ecrire(path, contenu string) {
	b.t.Helper()
	if err := os.WriteFile(path, []byte(contenu), 0600); err != nil {
		b.t.Fatal(err)
	}
}

func (b *banc) attendre(n int) {
	b.t.Helper()
	select {
	case got := <-b.imports:
		if got != n {
			b.t.Fatalf("import %d, attendu %d", got, n)
		}
	case <-time.After(3 * time.Second):
		b.t.Fatalf("import %d absent", n)
	}
}

func (b *banc) silence(d time.Duration) {
	b.t.Helper()
	// La phase d'amorçage peut encore produire un import déjà en attente.
	timer := time.NewTimer(d)
	defer timer.Stop()
	limite := time.NewTimer(3 * time.Second)
	defer limite.Stop()
	for {
		select {
		case <-b.imports:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(d)
		case <-timer.C:
			return
		case <-limite.C:
			b.t.Fatal("amorçage sans stabilisation")
		}
	}
}

func (b *banc) aucun() {
	b.t.Helper()
	select {
	case n := <-b.imports:
		b.t.Fatalf("import inattendu : %d", n)
	case <-time.After(3 * delaiTest):
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
}

func TestChangements(t *testing.T) {
	notes := []string{"note", "lien", "session", "journal", "recherche"}
	commits := []string{"commit_git", "fichier_modifie", "activite_jour", "mesure_langage"}
	for _, tc := range []struct {
		c      Changements
		tables []string
	}{
		{Changements{}, nil}, {Changements{Notes: 1}, notes}, {Changements{Commits: 1}, commits},
		{Changements{Notes: 2, Commits: 3}, append(append([]string{}, notes...), commits...)},
	} {
		if got := tc.c.Tables(); !reflect.DeepEqual(got, tc.tables) {
			t.Fatalf("Tables : %v", got)
		}
		if tc.c.Vide() != (len(tc.tables) == 0) {
			t.Fatal("Vide incohérent")
		}
	}
}

func TestNotesEtRegroupement(t *testing.T) {
	b := nouveauBanc(t)
	b.ecrire(filepath.Join(b.root, "note.md"), "une note")
	b.attendre(1)
	b.aucun()
	for i := 0; i < 20; i++ {
		b.ecrire(filepath.Join(b.root, "note.md"), "modification")
	}
	b.attendre(2)
	b.aucun()
}

func TestIgnorerEtNouveauxDossiers(t *testing.T) {
	b := nouveauBanc(t)
	for _, dir := range []string{".obsidian", ".trash", ".git", ".cache"} {
		mkdir(t, filepath.Join(b.root, dir))
		b.ecrire(filepath.Join(b.root, dir, "cache.md"), "ignoré")
	}
	b.ecrire(filepath.Join(b.root, ".obsidian", "x.json"), "{}")
	b.ecrire(filepath.Join(b.root, "note.txt"), "ignoré")
	b.aucun()
	sub := filepath.Join(b.root, "nouveau", "profond")
	mkdir(t, sub)
	b.ecrire(filepath.Join(sub, "note.md"), "nouvelle note")
	b.attendre(1)
	b.aucun()
	b.ecrire(filepath.Join(sub, "note.md"), "modification ultérieure")
	b.attendre(2)
	b.aucun()
}

func TestGit(t *testing.T) {
	depot := t.TempDir()
	git := filepath.Join(depot, ".git")
	mkdir(t, filepath.Join(git, "refs", "heads"))
	mkdir(t, filepath.Join(git, "objects"))
	b := nouveauBanc(t, depot)
	for _, name := range []string{"index.lock", "objects/objet", "refs/heads/main.lock"} {
		b.ecrire(filepath.Join(git, filepath.FromSlash(name)), "ignoré")
	}
	b.aucun()
	for i, name := range []string{"refs/heads/main", "HEAD", "packed-refs", "refs/heads/equipe/branche"} {
		path := filepath.Join(git, filepath.FromSlash(name))
		mkdir(t, filepath.Dir(path))
		b.ecrire(path, "nouvelle référence")
		b.attendre(i + 1)
		b.aucun()
	}
}

func TestImportLent(t *testing.T) {
	b := nouveauBanc(t)
	liberer := make(chan struct{})
	var actifs, maximum atomic.Int32
	b.mu.Lock()
	b.traiter = func(ctx context.Context, n int) (Changements, error) {
		a := actifs.Add(1)
		defer actifs.Add(-1)
		if a > maximum.Load() {
			maximum.Store(a)
		}
		if n == 1 {
			select {
			case <-liberer:
			case <-ctx.Done():
			}
		}
		return Changements{}, nil
	}
	b.mu.Unlock()
	b.ecrire(filepath.Join(b.root, "lent.md"), "premier")
	b.attendre(1)
	for i := 0; i < 20; i++ {
		b.ecrire(filepath.Join(b.root, "pendant.md"), "pendant")
	}
	b.aucun()
	close(liberer)
	b.attendre(2)
	b.aucun()
	if maximum.Load() != 1 {
		t.Fatal("imports concurrents")
	}
}

func TestErreurAnnonceEtDepotsDynamiques(t *testing.T) {
	ancien, nouveau := t.TempDir(), t.TempDir()
	for _, depot := range []string{ancien, nouveau} {
		mkdir(t, filepath.Join(depot, ".git", "refs", "heads"))
	}
	b := nouveauBanc(t, ancien)
	annonces := make(chan []string, 4)
	b.mu.Lock()
	b.annoncer = func(tables ...string) { annonces <- tables }
	b.traiter = func(ctx context.Context, n int) (Changements, error) {
		if n == 1 {
			return Changements{Notes: 1}, errors.New("échec simulé")
		}
		if n == 2 {
			return Changements{Notes: 1, Commits: 1}, nil
		}
		return Changements{}, nil
	}
	b.depotPaths = []string{nouveau}
	b.mu.Unlock()
	b.ecrire(filepath.Join(b.root, "note.md"), "erreur")
	b.attendre(1)
	b.aucun()
	select {
	case <-b.logs:
	case <-time.After(3 * time.Second):
		t.Fatal("erreur non journalisée")
	}
	select {
	case <-annonces:
		t.Fatal("annonce après erreur")
	default:
	}
	b.ecrire(filepath.Join(ancien, ".git", "HEAD"), "ancien")
	b.aucun()
	b.ecrire(filepath.Join(nouveau, ".git", "HEAD"), "nouveau")
	b.attendre(2)
	select {
	case tables := <-annonces:
		if !reflect.DeepEqual(tables, (Changements{Notes: 1, Commits: 1}).Tables()) {
			t.Fatalf("annonce : %v", tables)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("annonce absente")
	}
	b.aucun()
	b.ecrire(filepath.Join(b.root, "note.md"), "sans changement")
	b.attendre(3)
	b.aucun()
	select {
	case <-annonces:
		t.Fatal("annonce vide")
	default:
	}
}

func TestGitdirEtRepertoireCommun(t *testing.T) {
	depot, commun := t.TempDir(), t.TempDir()
	git := filepath.Join(commun, "worktrees", "test")
	mkdir(t, git)
	mkdir(t, filepath.Join(commun, "refs", "heads"))
	if err := os.WriteFile(filepath.Join(depot, ".git"), []byte("gitdir: "+git+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git, "commondir"), []byte("../..\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b := nouveauBanc(t, depot)
	b.ecrire(filepath.Join(git, "HEAD"), "checkout")
	b.attendre(1)
	b.aucun()
	b.ecrire(filepath.Join(commun, "refs", "heads", "main"), "commit")
	b.attendre(2)
	b.aucun()
}

func TestAnnulationEtValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Surveiller(ctx, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := Surveiller(context.Background(), Options{}); err == nil {
		t.Fatal("Importer requis")
	}
	if err := Surveiller(context.Background(), Options{Delai: -1, Importer: func(context.Context) (Changements, error) { return Changements{}, nil }}); err == nil {
		t.Fatal("délai négatif accepté")
	}
	b := nouveauBanc(t)
	b.ecrire(filepath.Join(b.root, "en-attente.md"), "annulation")
	b.cancel()
}

func TestGitCreationRefsEtGitdirRelatif(t *testing.T) {
	depot := t.TempDir()
	git := filepath.Join(depot, "metadonnees")
	mkdir(t, git)
	if err := os.WriteFile(filepath.Join(depot, ".git"), []byte("gitdir: metadonnees\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b := nouveauBanc(t, depot)
	branche := filepath.Join(git, "refs", "heads", "main")
	mkdir(t, filepath.Dir(branche))
	b.ecrire(branche, "premier commit")
	b.attendre(1)
	b.aucun()
	b.ecrire(branche, "second commit")
	b.attendre(2)
	b.aucun()
}

func TestAnnulationPendantImport(t *testing.T) {
	b := nouveauBanc(t)
	arrete := make(chan struct{})
	b.mu.Lock()
	b.traiter = func(ctx context.Context, n int) (Changements, error) {
		<-ctx.Done()
		close(arrete)
		return Changements{}, ctx.Err()
	}
	b.mu.Unlock()
	b.ecrire(filepath.Join(b.root, "note.md"), "import bloqué")
	b.attendre(1)
	b.cancel()
	select {
	case <-arrete:
	case <-time.After(3 * time.Second):
		t.Fatal("import non annulé")
	}
}

func TestFenetreDepuisPremierEvenement(t *testing.T) {
	b := nouveauBanc(t)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	limite := time.NewTimer(time.Second)
	defer limite.Stop()
	for {
		select {
		case <-ticker.C:
			b.ecrire(filepath.Join(b.root, "continu.md"), "écritures continues")
		case <-b.imports:
			return
		case <-limite.C:
			t.Fatal("fenêtre repoussée par les écritures suivantes")
		}
	}
}

func TestRenommageEtSuppressionNotes(t *testing.T) {
	b := nouveauBanc(t)
	ancien := filepath.Join(b.root, "ancien.md")
	nouveau := filepath.Join(b.root, "nouveau.md")
	b.ecrire(ancien, "note")
	b.attendre(1)
	b.aucun()
	if err := os.Rename(ancien, nouveau); err != nil {
		t.Fatal(err)
	}
	b.attendre(2)
	b.aucun()
	if err := os.Remove(nouveau); err != nil {
		t.Fatal(err)
	}
	b.attendre(3)
	b.aucun()
}
