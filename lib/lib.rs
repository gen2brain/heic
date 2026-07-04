use std::alloc::{alloc, dealloc, Layout};

use heic::{DecoderConfig, ImageInfo, PixelLayout};

const HDR: usize = 8;

#[no_mangle]
pub extern "C" fn malloc(size: usize) -> *mut u8 {
    if size == 0 {
        return std::ptr::null_mut();
    }
    let total = size + HDR;
    let layout = Layout::from_size_align(total, HDR).unwrap();
    unsafe {
        let p = alloc(layout);
        if p.is_null() {
            return p;
        }
        (p as *mut usize).write(total);
        p.add(HDR)
    }
}

#[no_mangle]
pub extern "C" fn free(ptr: *mut u8) {
    if ptr.is_null() {
        return;
    }
    unsafe {
        let base = ptr.sub(HDR);
        let total = (base as *mut usize).read();
        dealloc(base, Layout::from_size_align(total, HDR).unwrap());
    }
}

#[no_mangle]
pub extern "C" fn decode(in_ptr: *const u8, in_len: i32, config_only: i32, info: *mut u32) -> *mut u8 {
    let input = unsafe { std::slice::from_raw_parts(in_ptr, in_len as usize) };

    if config_only != 0 {
        match ImageInfo::from_bytes(input) {
            Ok(i) => unsafe {
                *info.add(0) = i.width;
                *info.add(1) = i.height;
            },
            Err(_) => unsafe { *info.add(0) = 0 },
        }
        return std::ptr::null_mut();
    }

    let out = match DecoderConfig::new().decode(input, PixelLayout::Rgba8) {
        Ok(o) => o,
        Err(_) => return std::ptr::null_mut(),
    };

    unsafe {
        *info.add(0) = out.width;
        *info.add(1) = out.height;
    }

    let size = out.data.len();
    let p = malloc(size);
    if p.is_null() {
        return p;
    }
    unsafe { std::ptr::copy_nonoverlapping(out.data.as_ptr(), p, size) };
    p
}

#[no_mangle]
pub extern "C" fn decode_sequence(in_ptr: *const u8, in_len: i32, info: *mut u32) -> *mut u8 {
    let input = unsafe { std::slice::from_raw_parts(in_ptr, in_len as usize) };

    let mut dec = heic::VideoDecoder::new(16);
    let frames = match dec.decode_annex_b(input) {
        Ok(f) => f,
        Err(_) => return std::ptr::null_mut(),
    };

    if frames.is_empty() {
        unsafe { *info.add(2) = 0 };
        return std::ptr::null_mut();
    }

    let mut buffers: Vec<Vec<u8>> = Vec::with_capacity(frames.len());
    for f in &frames {
        match f.to_rgba() {
            Ok(r) => buffers.push(r),
            Err(_) => return std::ptr::null_mut(),
        }
    }

    let frame_size = buffers[0].len();
    unsafe {
        *info.add(0) = frames[0].cropped_width();
        *info.add(1) = frames[0].cropped_height();
        *info.add(2) = frames.len() as u32;
    }

    let total = frame_size * buffers.len();
    let p = malloc(total);
    if p.is_null() {
        return p;
    }

    let out = unsafe { std::slice::from_raw_parts_mut(p, total) };
    for (i, buf) in buffers.iter().enumerate() {
        let off = i * frame_size;
        let n = buf.len().min(frame_size);
        out[off..off + n].copy_from_slice(&buf[..n]);
    }

    p
}
