//go:build beyla_bpf_ignore
#include "k_tracer.h"
#include "http_ssl.h"
#include "nodejs.h"

#include "flow.h"

char __license[] SEC("license") = "Dual MIT/GPL";

SEC("kprobe/sys_recvfrom")
int BPF_KPROBE(beyla_kprobe_sys_recvfrom) {
    u64 id = bpf_get_current_pid_tgid();

    if (!valid_pid(id)) {
        return 0;
    }

    bpf_dbg_printk("=== tcp connect %llx ===", id);

    return 0;
}