enable wgpu_ray_query;

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coords: vec2<f32>,
};

@vertex
fn vs_main(@builtin(vertex_index) vertex_index: u32) -> VertexOutput {
    var result: VertexOutput;
    let x = i32(vertex_index) / 2;
    let y = i32(vertex_index) & 1;
    let tc = vec2<f32>(
        f32(x) * 2.0,
        f32(y) * 2.0
    );
    result.position = vec4<f32>(
        tc.x * 2.0 - 1.0,
        1.0 - tc.y * 2.0,
        0.0, 1.0
    );
    result.tex_coords = tc;
    return result;
}

/*
The contents of the RayQuery struct are roughly as follows
let RAY_FLAG_NONE = 0x00u;
let RAY_FLAG_OPAQUE = 0x01u;
let RAY_FLAG_NO_OPAQUE = 0x02u;
let RAY_FLAG_TERMINATE_ON_FIRST_HIT = 0x04u;
let RAY_FLAG_SKIP_CLOSEST_HIT_SHADER = 0x08u;
let RAY_FLAG_CULL_BACK_FACING = 0x10u;
let RAY_FLAG_CULL_FRONT_FACING = 0x20u;
let RAY_FLAG_CULL_OPAQUE = 0x40u;
let RAY_FLAG_CULL_NO_OPAQUE = 0x80u;
let RAY_FLAG_SKIP_TRIANGLES = 0x100u;
let RAY_FLAG_SKIP_AABBS = 0x200u;

let RAY_QUERY_INTERSECTION_NONE = 0u;
let RAY_QUERY_INTERSECTION_TRIANGLE = 1u;
let RAY_QUERY_INTERSECTION_GENERATED = 2u;
let RAY_QUERY_INTERSECTION_AABB = 3u;

struct RayDesc {
    flags: u32,
    cull_mask: u32,
    t_min: f32,
    t_max: f32,
    origin: vec3<f32>,
    dir: vec3<f32>,
}

struct RayIntersection {
    kind: u32,
    t: f32,
    instance_custom_data: u32,
    instance_index: u32,
    sbt_record_offset: u32,
    geometry_index: u32,
    primitive_index: u32,
    barycentrics: vec2<f32>,
    front_face: bool,
    object_to_world: mat4x3<f32>,
    world_to_object: mat4x3<f32>,
}
*/

struct Uniforms {
    view_inv: mat4x4<f32>,
    proj_inv: mat4x4<f32>,

    // Camera position in world space.
    cam_pos: vec3<f32>,
    // Orthonormal transform from aperture space to world space.
    // * Column 0 is the eye vector
    // * Column 1 the left vector
    // * Column 2 the up vector.
    cam_aperture_to_world: mat3x3<f32>,

    cam_aperture_length: f32,
    cam_aperture_x: f32,
    cam_aperture_y: f32,
};

struct Vertex {
    pos: vec3<f32>,
    normal: vec3<f32>,
    uv: vec2<f32>,
};


struct Instance {
    first_vertex: u32,
    first_geometry: u32,
    last_geometry: u32,
    _pad: u32
};

struct Material{
    roughness_exponent: f32,
    metalness: f32,
    specularity: f32,
    albedo: vec3<f32>
}

struct Geometry {
    first_index: u32,
    material: Material,
};


@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(0) @binding(1)
var<storage, read> vertices: array<Vertex>;

@group(0) @binding(2)
var<storage, read> indices: array<u32>;

@group(0) @binding(3)
var<storage, read> geometries: array<Geometry>;

@group(0) @binding(4)
var<storage, read> instances: array<Instance>;

@group(0) @binding(5)
var acc_struct: acceleration_structure;

@fragment
fn fs_main(vertex: VertexOutput) -> @location(0) vec4<f32> {
	let d = vertex.tex_coords * 2.0 - 1.0;

	let origin = (uniforms.view_inv * vec4<f32>(0.0,0.0,0.0,1.0)).xyz;
	let temp = uniforms.proj_inv * vec4<f32>(d.x, d.y, 1.0, 1.0);
	let direction = (uniforms.view_inv * vec4<f32>(normalize(temp.xyz), 0.0)).xyz;

    var rq: ray_query;
    rayQueryInitialize(&rq, acc_struct, RayDesc(0u, 0xFFu, 0.1, 200.0, origin, direction));
    rayQueryProceed(&rq);

    let intersection = rayQueryGetCommittedIntersection(&rq);
    if (intersection.kind != RAY_QUERY_INTERSECTION_NONE) {
        let instance = instances[intersection.instance_custom_data];
        let geometry = geometries[intersection.geometry_index + instance.first_geometry];

        let index_offset = geometry.first_index;
        let vertex_offset = instance.first_vertex;

        let first_index_index = intersection.primitive_index * 3u + index_offset;

        let v_0 = vertices[vertex_offset+indices[first_index_index+0u]];
        let v_1 = vertices[vertex_offset+indices[first_index_index+1u]];
        let v_2 = vertices[vertex_offset+indices[first_index_index+2u]];

        let bary = vec3<f32>(1.0 - intersection.barycentrics.x - intersection.barycentrics.y, intersection.barycentrics);

        let pos = v_0.pos * bary.x + v_1.pos * bary.y + v_2.pos * bary.z;
        let normal_raw = v_0.normal * bary.x + v_1.normal * bary.y + v_2.normal * bary.z;
        let uv = v_0.uv * bary.x + v_1.uv * bary.y + v_2.uv * bary.z;

        let normal = normalize(normal_raw);

        let material = geometry.material;

        let global_contact_point = origin + intersection.t * direction;
        let global_normal : vec3<f32> = normalize(intersection.object_to_world * normal);

        var light_contribution = f32(0.0);

        // Ambient lighting term.
        light_contribution += 0.3;

        // TODO: There is still some weirdness --- triangles that rotate from
        // occluded to not-occluded "pop" very suddenly between dark and light.
        // I would expect that they should instead smoothly transition due to
        // the dot product in the lighting term.

        // Fire an occlusion ray towards the directional light.
        let directional_light_direction = normalize(vec3<f32>(0, -1, 0));
        if (dot(global_normal, directional_light_direction) > 0.0) {
            var directional_shadow_rq: ray_query;
            rayQueryInitialize(
                &directional_shadow_rq,
                acc_struct,
                RayDesc(0u, 0xFFu, 0.001, 1000.0, global_contact_point+global_normal*0.001, directional_light_direction),
            );
            rayQueryProceed(&directional_shadow_rq);
            let directional_shadow_intersection = rayQueryGetCommittedIntersection(&directional_shadow_rq);
            if (directional_shadow_intersection.kind == RAY_QUERY_INTERSECTION_NONE) {
                // Contact point is visible to this light
                light_contribution += 0.7 * dot(global_normal, directional_light_direction);
            }
        }

        return vec4<f32>(light_contribution * material.albedo.x, light_contribution * material.albedo.y, light_contribution*material.albedo.z, 1);
    }

    return vec4<f32>(0, 0, 0, 1);
}
