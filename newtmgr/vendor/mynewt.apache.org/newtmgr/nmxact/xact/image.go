package xact

import (
	"fmt"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

//////////////////////////////////////////////////////////////////////////////
// $upload                                                                  //
//////////////////////////////////////////////////////////////////////////////

type ImageUploadProgressFn func(c *ImageUploadCmd, r *nmp.ImageUploadRsp)
type ImageUploadCmd struct {
	CmdBase
	Data       []byte
	ProgressCb ImageUploadProgressFn
}

type ImageUploadResult struct {
	Rsps []*nmp.ImageUploadRsp
}

func NewImageUploadCmd() *ImageUploadCmd {
	return &ImageUploadCmd{
		CmdBase: NewCmdBase(),
	}
}

func newImageUploadResult() *ImageUploadResult {
	return &ImageUploadResult{}
}

func (r *ImageUploadResult) Status() int {
	if len(r.Rsps) > 0 {
		return r.Rsps[len(r.Rsps)-1].Rc
	} else {
		return nmp.NMP_ERR_EUNKNOWN
	}
}

func buildImageUploadReq(imageSz int, chunk []byte,
	off int) *nmp.ImageUploadReq {

	r := nmp.NewImageUploadReq()

	if r.Off == 0 {
		r.Len = uint32(imageSz)
	}
	r.Off = uint32(off)
	r.Data = chunk

	return r
}

func nextImageUploadReq(s sesn.Sesn, data []byte, off int) (
	*nmp.ImageUploadReq, error) {

	// First, build a request without data to determine how much data could
	// fit.
	empty := buildImageUploadReq(len(data), nil, off)
	emptyEnc, err := s.EncodeNmpMsg(empty.Msg())
	if err != nil {
		return nil, err
	}

	room := s.MtuOut() - len(emptyEnc)
	if room <= 0 {
		return nil, fmt.Errorf("Cannot create image upload request; "+
			"MTU too low to fit any image data; max-payload-size=%d",
			s.MtuOut())
	}

	if off+room > len(data) {
		// Final chunk.
		room = len(data) - off
	}

	// Assume all the unused space can hold image data.  This assumption may
	// not be valid for some encodings (e.g., CBOR uses variable length fields
	// to encodes byte string lengths).
	r := buildImageUploadReq(len(data), data[off:off+room], off)
	enc, err := s.EncodeNmpMsg(r.Msg())
	if err != nil {
		return nil, err
	}

	oversize := len(enc) - s.MtuOut()
	if oversize > 0 {
		// Request too big.  Reduce the amount of image data.
		r = buildImageUploadReq(len(data), data[off:off+room-oversize], off)
	}

	return r, nil
}

func (c *ImageUploadCmd) Run(s sesn.Sesn) (Result, error) {
	res := newImageUploadResult()

	for off := 0; off < len(c.Data); {
		r, err := nextImageUploadReq(s, c.Data, off)
		if err != nil {
			return nil, err
		}

		rsp, err := txReq(s, r.Msg(), &c.CmdBase)
		if err != nil {
			return nil, err
		}
		irsp := rsp.(*nmp.ImageUploadRsp)

		off = int(irsp.Off)

		if c.ProgressCb != nil {
			c.ProgressCb(c, irsp)
		}

		res.Rsps = append(res.Rsps, irsp)
		if irsp.Rc != 0 {
			break
		}
	}

	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $upgrade                                                                 //
//////////////////////////////////////////////////////////////////////////////

// Image upgrade combines the image erase and image upload commands into a
// single command.  Some hardware and / or BLE connection settings cause the
// connection to drop during the initial erase.  The image upgrade command
// addresses this issue with the following sequence:
// 1. Send image erase command.
// 2. If the image erase command succeeded, proceed to step 5.
// 3. Else if the peer is disconnected, attempt to reconnect to the peer.  If
//    the reconnect attempt fails, abort the command and report the error.  If
//    the reconnect attempt succeeded, proceed to step 5.
// 4. Else (the erase command failed and the peer is still connected), proceed
//    to step 5.
// 5. Execute the upload command.

type ImageUpgradeCmd struct {
	CmdBase
	Data       []byte
	ProgressCb ImageUploadProgressFn
}

type ImageUpgradeResult struct {
	EraseRes  *ImageEraseResult
	UploadRes *ImageUploadResult
}

func NewImageUpgradeCmd() *ImageUpgradeCmd {
	return &ImageUpgradeCmd{
		CmdBase: NewCmdBase(),
	}
}

func newImageUpgradeResult() *ImageUpgradeResult {
	return &ImageUpgradeResult{}
}

func (r *ImageUpgradeResult) Status() int {
	if r.UploadRes != nil {
		return r.UploadRes.Status()
	} else if r.EraseRes != nil {
		return r.EraseRes.Status()
	} else {
		return nmp.NMP_ERR_EUNKNOWN
	}
}

func (c *ImageUpgradeCmd) Run(s sesn.Sesn) (Result, error) {
	upgradeRes := newImageUpgradeResult()

	eraseCmd := NewImageEraseCmd()
	res, err := eraseCmd.Run(s)

	if res != nil {
		upgradeRes.EraseRes = res.(*ImageEraseResult)
	}
	if err != nil || res.Status() != 0 {
		// If the erase command failed and the peer is no longer connected, the
		// disconnect could have been caused by a stall in the destination
		// device's processor.  In this case, the erase command succeeded.  Try
		// to recover by reconnecting.
		if !s.IsOpen() {
			if err := s.Open(); err != nil {
				return upgradeRes, err
			}
		}
	}

	uploadCmd := NewImageUploadCmd()
	uploadCmd.Data = c.Data
	uploadCmd.ProgressCb = c.ProgressCb
	res, err = uploadCmd.Run(s)

	if res != nil {
		upgradeRes.UploadRes = res.(*ImageUploadResult)
	}
	return upgradeRes, err
}

//////////////////////////////////////////////////////////////////////////////
// $state read                                                              //
//////////////////////////////////////////////////////////////////////////////

type ImageStateReadCmd struct {
	CmdBase
}

type ImageStateReadResult struct {
	Rsp *nmp.ImageStateRsp
}

func NewImageStateReadCmd() *ImageStateReadCmd {
	return &ImageStateReadCmd{
		CmdBase: NewCmdBase(),
	}
}

func newImageStateReadResult() *ImageStateReadResult {
	return &ImageStateReadResult{}
}

func (r *ImageStateReadResult) Status() int {
	return r.Rsp.Rc
}

func (c *ImageStateReadCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewImageStateReadReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ImageStateRsp)

	res := newImageStateReadResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $state write                                                             //
//////////////////////////////////////////////////////////////////////////////

type ImageStateWriteCmd struct {
	CmdBase
	Hash    []byte
	Confirm bool
}

type ImageStateWriteResult struct {
	Rsp *nmp.ImageStateRsp
}

func NewImageStateWriteCmd() *ImageStateWriteCmd {
	return &ImageStateWriteCmd{
		CmdBase: NewCmdBase(),
	}
}

func newImageStateWriteResult() *ImageStateWriteResult {
	return &ImageStateWriteResult{}
}

func (r *ImageStateWriteResult) Status() int {
	return r.Rsp.Rc
}

func (c *ImageStateWriteCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewImageStateWriteReq()
	r.Hash = c.Hash
	r.Confirm = c.Confirm

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ImageStateRsp)

	res := newImageStateWriteResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $corelist                                                                //
//////////////////////////////////////////////////////////////////////////////

type CoreListCmd struct {
	CmdBase
}

type CoreListResult struct {
	Rsp *nmp.CoreListRsp
}

func NewCoreListCmd() *CoreListCmd {
	return &CoreListCmd{
		CmdBase: NewCmdBase(),
	}
}

func newCoreListResult() *CoreListResult {
	return &CoreListResult{}
}

func (r *CoreListResult) Status() int {
	return r.Rsp.Rc
}

func (c *CoreListCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewCoreListReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.CoreListRsp)

	res := newCoreListResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $erase                                                                   //
//////////////////////////////////////////////////////////////////////////////

type ImageEraseCmd struct {
	CmdBase
}

type ImageEraseResult struct {
	Rsp *nmp.ImageEraseRsp
}

func NewImageEraseCmd() *ImageEraseCmd {
	return &ImageEraseCmd{
		CmdBase: NewCmdBase(),
	}
}

func newImageEraseResult() *ImageEraseResult {
	return &ImageEraseResult{}
}

func (r *ImageEraseResult) Status() int {
	return r.Rsp.Rc
}

func (c *ImageEraseCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewImageEraseReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ImageEraseRsp)

	res := newImageEraseResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $coreload                                                                //
//////////////////////////////////////////////////////////////////////////////

type CoreLoadProgressFn func(c *CoreLoadCmd, r *nmp.CoreLoadRsp)
type CoreLoadCmd struct {
	CmdBase
	ProgressCb CoreLoadProgressFn
}

type CoreLoadResult struct {
	Rsps []*nmp.CoreLoadRsp
}

func NewCoreLoadCmd() *CoreLoadCmd {
	return &CoreLoadCmd{
		CmdBase: NewCmdBase(),
	}
}

func newCoreLoadResult() *CoreLoadResult {
	return &CoreLoadResult{}
}

func (r *CoreLoadResult) Status() int {
	rsp := r.Rsps[len(r.Rsps)-1]
	return rsp.Rc
}

func (c *CoreLoadCmd) Run(s sesn.Sesn) (Result, error) {
	res := newCoreLoadResult()
	off := 0

	for {
		r := nmp.NewCoreLoadReq()
		r.Off = uint32(off)

		rsp, err := txReq(s, r.Msg(), &c.CmdBase)
		if err != nil {
			return nil, err
		}
		irsp := rsp.(*nmp.CoreLoadRsp)

		if c.ProgressCb != nil {
			c.ProgressCb(c, irsp)
		}

		res.Rsps = append(res.Rsps, irsp)
		if irsp.Rc != 0 {
			break
		}

		if len(irsp.Data) == 0 {
			// Download complete.
			break
		}

		off = int(irsp.Off) + len(irsp.Data)
	}

	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $coreerase                                                               //
//////////////////////////////////////////////////////////////////////////////

type CoreEraseCmd struct {
	CmdBase
}

type CoreEraseResult struct {
	Rsp *nmp.CoreEraseRsp
}

func NewCoreEraseCmd() *CoreEraseCmd {
	return &CoreEraseCmd{
		CmdBase: NewCmdBase(),
	}
}

func newCoreEraseResult() *CoreEraseResult {
	return &CoreEraseResult{}
}

func (r *CoreEraseResult) Status() int {
	return r.Rsp.Rc
}

func (c *CoreEraseCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewCoreEraseReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.CoreEraseRsp)

	res := newCoreEraseResult()
	res.Rsp = srsp
	return res, nil
}
