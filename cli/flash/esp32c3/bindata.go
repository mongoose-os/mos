// Code generated for package esp32c3 by go-bindata DO NOT EDIT. (@generated)
// sources:
// stub/stub.json
package esp32c3

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)
type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _stubStubJson = []byte(`{"params_start": 1077411840, "code": "37070c603c4f411106c693f707c03ccf130580029700c8ffe780c05673d0007e73d0107e9700c8ffe780006f81459700c8ffe780e012372700605c4bb240b7060080d58f5ccb410182809967aa97c207dc4b4111416722c47d1713944700d183798cbd8b3e94b305b4021375f50f06c6b3d5c5029700c8ffe780c0002285b240224441018280b787c93f9387070011673e971457056693058600b386b602b697d44363fbc60013861600d0c3b6972384a7005c5785075cd782807971b707006022d452cc3784c93f03aa87004ad04ece5ac837c9c93f93090400116b06d626d256ca5ec6130909034e9bb7070060c44f93f4f43fc5c8814bb7cac93fb707006088430327040091471375f50f63e0e70293172700ca979c4382879307000c6317f50085472320f40023260b02850be39774fd65bf9307b00d6316f5008d472320f400edb79306000c83278b021306040013880a00631dd5061387170091466302d7062324eb02032788028566938586003307b7023297232207003697230407000327c80231e3b387b702b297be9683c78600858b95eb91472320f400b707006023a60700b2502254b707006023a8470192540259f249624ad24a424bb24b4561828023240b0245b72320040085b78546631dd700056793068700b387d702ce97ba972384a700894799bf713d81b79307c00d6309f5009307d00de31ef5f81305b00d19a01305000cbd3df1bf35714ac97d73056922cd26cb4ec756c35ac1dede7d7406cf52c5930709071a91930984fa8a97be99aa8b4e85ae8a328b9700c8ffe780003d130709070a9733098700130484f981443a9463ed54030567930707077d74930584fa8a97130484f93e94930707078a97be9522859700c8ffe780c0392285c1459700c8ffe780c0e10145b1a8338a9a4063734b015a8a5286ca85338574019700c8ffe78060e815edd2854a859700c8ffe78000df91452285232c09f8ef00607b9147631ff502032689f9e3f4c4f8058eca854e859700c8ffe7808033832489f98dbf1305300505631a91fa406a44da444a49ba492a4a9a4a0a4bf65b0d61828013054005cdb70111b707ce3f22cc2a8403a507ff2ec606ce9700c8ffe78080deb7270060984fb70600e0fd16758f98cfd84fb7060004fd16758fb706005c558fb245d8cf3707004098c398437dffc0c3f24062448cc3014505618280297123263113aa898808232c8112232c8111232e1112232a911223282113232441132322511323206113232e7111232a91112328a1112326b1112e8c32c69700c8ffe780e0268567fd1733f7f90013042003631b072eb377fc00130430036395072e9700c8ffe78060d72a8913044003631c052c378ac93f116613060603814513050a009700c8ffe78080f63704c93f21668145130504009700c8ffe78040f5b705384001469385250c15459700c8ffe780001fb7070060d853130500023ac23707340013074706d8d313071010d8c79700c8ffe780401d13064002814568109700c8ffe780c0f06146814548089700c8ffe780e0efe1454808112df327207ec16c22c03ede0564130b0a00930b0a00d24763e387038c0888009700c8ffe780a019f327207e7257930540026810998f3ede0144e92bfdaaf32a207ec167fd17b3f4f900b24789c7d247a2976363f9067327207ee257b38a5741ba9a56dc7326207eb797c93f93870700dc4703274b008966da963e97dc4a8d66da96d44e3e9783270a0036973acc11675a974453b7cac93f37172707938a0a009145130707e083a68a0263949604639ab7021304800351a2b3072c4163eb970189e8b70580004e85e533631605166699adbfb70500014e85e13b6317051a2299adb7f326207e918ee37fd7fa13047003a9a21147e38fe7fa7327207eb257918fba973ed6f327207e130784003387e4023ec4930787005e97b306e40003c68600b38dfb0093761600e9ce05669305f00f6e859700c8ffe78000dc056d7326207e4257a247ee8588081d8f32976a863ad89700c8ffe7806006130784003387e4025e97229703478700058b31e77326207e3707ce3f032507ff32c49700c8ffe78000b37327207ee25522462e97118f3adc7328207e6a86ee854e8542c49700c8ffe780e0b265e97327207ed2572248b3870741ba973eda93078400b384f40283a74a0211478507de9423a204006382e70623a2fa02d247130680058c08b386a701281136ca9700c8ffe78020d12c1168089700c8ffe780a0fce1454808ea99192919bd098a032d470015da8247ea860547138507006e86a1659700c8ffe78060a8fd572a8d6305f5068247938d070039b723a20a0245b7130450031247b707006013050002d8d39700c8ffe78000f622858320c1130324811383244113032901138329c112032a8112832a4112032b0112832bc111032c8111832c4111032d0111832dc11031618280130460037db71304900365b7130400044db7856763e3c70009be1305200582805171cecfd6cbdec786d7a2d5a6d3cad1d2cddac9e2c57d731a91aa8bae89b28a0dc6856713052006328b63f1c70205631a91be502e549e540e59fe496e4ade4a4e4bbe4b2e4c6d618280056b05697d749307090c930404f58a97be9426859700c8ffe780c0ea1307090c130a84fa0a973a9a1307090c0a97330c8700130404f43a94639d090205679307070c7d74930504f58a97130404f43e949307070c8a97be9522859700c8ffe780e0e62285c1459700c8ffe780e08e014595bf52859700c8ffe780c0e44e8963733b015a894a86e2855e859700c8ffe780209501c513053006b1b74a86e28526859700c8ffe78040e263850a024a86e28552859700c8ffe78020e1d28522859700c8ffe780a0e0c14522859700c8ffe780a088ca9bb389294185b7b72700603707001098c3984393163700e3cd06fea84fb7070001fd177d8d8280011106cef13f2ac6914568009700c8ffe780e084f240014505618280797126d24ad093075005817437c9c93f22d406d62305f10037040060fd14130949041c50370702008545d98f1cd01c501305a100e58f1cd0f12a85476311f504fd57a305f1008347a1002547fd1793f7f70f6362f7028a07ca979c438287c14508084d2ab1476316f5026246d24542457934a305a10085451305b100412a0345a10005479307a5ff93f7f70fe36bf7f88da093071004a305f100f1bfc1450808952ab1476317f5006246d2454245d533c9b793071005c5b7c1450808a12ab1476317f5006246d2454245d93b5db793071006d1b7013f71bf97f0c7ffe780e07d49bf1305b1008545a305010005220345a100b25022549254025945618280c1450808092aaa8508081122a3050100a5b7c1450808012291476318f500c247914568009c433ec6cdb7930710f9adb7c1450808cd20a1476316f500c247524798c3e9b7930710fa81bfb7073840011193870700014506ce22cc26cac043844397f0c7ffe780406feff08fe3c167fd171307001085664166b7050001014597f0c7ffe780e07402c611cc09651305057197f0c7ffe78080652286a6850145eff0cfe42ac631651305053597f0c7ffe780e06391456800a128a9352a8409651305057197f0c7ffe780606299476306f40097f0c7ffe7808065f2406244d244056182803705c93f37c6c93f93070500130606031d8e4111098681451305050006c69700c8ffe780c08eb24041013dbf17f3c7ff67002363797122d44ece52cc06d626d24ad056ca5ac85ec662c42a8aae891304000c97f0c7ffe780805d2a89e31b85fe0144930a000c130bb00d930bc00d130cd00d631734039304000c97f0c7ffe780005be31c95fe2285b250225492540259f249624ad24a424bb24b224c4561828097f0c7ffe780a058aa84e30e55fd631d650197f0c7ffe7808057630675016305850101444dbfca84b3078a0023809700050445b70000", "code_start": 1077411848, "entry": 1077414474, "data": "220138403c0138403c013840f6013840ac013840c808384006093840220938403e093840420938404c0938404c093840680938407a09384098093840", "data_start": 1070186544, "num_params": 2, "code_size": 2840, "data_size": 60}`)

func stubStubJsonBytes() ([]byte, error) {
	return _stubStubJson, nil
}

func stubStubJson() (*asset, error) {
	bytes, err := stubStubJsonBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "stub/stub.json", size: 5978, mode: os.FileMode(420), modTime: time.Unix(1, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"stub/stub.json": stubStubJson,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"stub": &bintree{nil, map[string]*bintree{
		"stub.json": &bintree{stubStubJson, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
